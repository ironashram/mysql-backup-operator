/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	m1kcloudv1alpha1 "github.com/ironashram/mysql-backup-operator/api/v1alpha1"
)

var log = ctrl.Log.WithName("controller")

// MysqlBackupReconciler reconciles a MysqlBackup object
type MysqlBackupReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=m1kcloud.m1k.cloud,resources=mysqlbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=m1kcloud.m1k.cloud,resources=mysqlbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile MysqlBackup CRD
func (r *MysqlBackupReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	// fetch mysqlbackup
	mysqlbackup := &m1kcloudv1alpha1.MysqlBackup{}
	err := r.Get(ctx, req.NamespacedName, mysqlbackup)
	if err != nil {
		if errors.IsNotFound(err) {
			// Return and don't requeue
			log.Info("MysqlBackup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get MysqlBackup")
		return ctrl.Result{}, err
	}

	// Get Status
	status := mysqlbackup.Status.BackupStatus

	// Add status to all logs
	log := log.WithValues("mysqlBackup", req.NamespacedName, "status", status)

	switch status {

	case "":
		log.Info("Start initial phase")

		// Check if job treshold per database is reached
		globalJobList := &batchv1.JobList{}
		globalListOpts := []client.ListOption{
			client.InNamespace(mysqlbackup.Namespace),
			client.MatchingLabels{"cluster": mysqlbackup.Spec.ClusterRef.ClusterName, "database": mysqlbackup.Spec.Database},
		}
		if err = r.List(ctx, globalJobList, globalListOpts...); err != nil {
			log.Error(err, "Failed to get Job list")
			return ctrl.Result{}, err
		}
		if len(globalJobList.Items) >= mysqlbackup.Spec.MaxJobs {
			log.Info("Max number of jobs per database reached")
			updateStatus(r, mysqlbackup, "NotReady")
			return ctrl.Result{Requeue: true}, nil
		}
		// Set creatingBackup status and requeue
		mysqlbackup.Status.BackupStatus = "createJob"
		err := r.Status().Update(ctx, mysqlbackup)
		if err != nil {
			log.Error(err, "Failed to update cr status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil

	case "createJob":
		log.Info("Start createJob phase")

		// Check if the Job already exists, if not create it
		jobs := &batchv1.JobList{}
		Opts := []client.ListOption{
			client.InNamespace(mysqlbackup.Namespace),
			client.MatchingLabels(joblabels(mysqlbackup.Name, mysqlbackup.Spec.ClusterRef.ClusterName, mysqlbackup.Spec.Database)),
		}
		if err = r.List(ctx, jobs, Opts...); err != nil {
			log.Error(err, "Failed to get Job")
			return ctrl.Result{}, err
		}

		if len(jobs.Items) == 0 {
			job := r.mysqlJob(mysqlbackup)
			log.Info("Creating a new Job")
			err = r.Create(ctx, job)
			if err != nil {
				log.Error(err, "Failed to create new Job")
				return ctrl.Result{}, err
			}
			log.Info("Job Created Successfully")
			updateStatus(r, mysqlbackup, "checkJob")
		} else if len(jobs.Items) > 0 {
			log.Info("Job already present")
			return ctrl.Result{Requeue: false}, nil
		}

		return ctrl.Result{Requeue: true}, nil

	case "checkJob":
		log.Info("Start checkJob phase")

		// List the jobs for this mysqlbackup cr
		jobList := &batchv1.JobList{}
		listOpts := []client.ListOption{
			client.InNamespace(mysqlbackup.Namespace),
			client.MatchingLabels(joblabels(mysqlbackup.Name, mysqlbackup.Spec.ClusterRef.ClusterName, mysqlbackup.Spec.Database)),
		}
		if err = r.List(ctx, jobList, listOpts...); err != nil {
			log.Error(err, "Failed to get Job list")
			return ctrl.Result{}, err
		}

		// Update mysqlbackup cr status with job results
		if len(jobList.Items) == 1 {
			// retrieve job status for all jobs
			jobStatus := getjobStatus(jobList.Items)
			for obj, objStatus := range jobStatus {
				if objStatus.Succeeded == 1 {
					if !contains(mysqlbackup.Spec.SuccessfulJobs, obj) {
						mysqlbackup.Spec.SuccessfulJobs = append(mysqlbackup.Spec.SuccessfulJobs, obj)
						mysqlbackup.Spec.JobCount = mysqlbackup.Spec.JobCount + 1
						log.Info("Job successful, adding to cr")
						err := r.Update(ctx, mysqlbackup)
						if err != nil {
							log.Error(err, "Failed to update cr")
							return ctrl.Result{}, err
						}
						updateStatus(r, mysqlbackup, "Ready")
						return ctrl.Result{Requeue: true}, nil
					}
				} else if objStatus.Failed == 1 {
					if !contains(mysqlbackup.Spec.FailedJobs, obj) {
						mysqlbackup.Spec.FailedJobs = append(mysqlbackup.Spec.FailedJobs, obj)
						mysqlbackup.Spec.JobCount = mysqlbackup.Spec.JobCount + 1
						log.Info("Job failed, adding to cr")
						err := r.Update(ctx, mysqlbackup)
						if err != nil {
							log.Error(err, "Failed to update cr")
							return ctrl.Result{}, err
						}
						updateStatus(r, mysqlbackup, "NotReady")
						return ctrl.Result{Requeue: true}, nil
					}
				} else if objStatus.Active == 1 {
					log.Info("Job still running")
					return ctrl.Result{Requeue: true}, nil
				}
			}
		} else if len(jobList.Items) > 1 {
			log.Info("Too many jobs, probable race condition")
			updateStatus(r, mysqlbackup, "NotReady")

			return ctrl.Result{Requeue: true}, nil
		}

	case "NotReady":
		log.Info("Start NotReady phase")

	case "Ready":
		log.Info("Start Ready phase")

	default:
		log.Info("Start Unknown state phase")
	}

	return ctrl.Result{}, nil
}

// mysqlJob returns a mysql job object
func (r *MysqlBackupReconciler) mysqlJob(m *m1kcloudv1alpha1.MysqlBackup) *batchv1.Job {
	ls := joblabels(m.Name, m.Spec.ClusterRef.ClusterName, m.Spec.Database)
	backoffLimit := int32(1)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "mysqljob-",
			Namespace:    m.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "OnFailure",
					Containers: []corev1.Container{{
						Name:    "mysqldump",
						Image:   "quay.io/ironashram/test-alpine:v0.0.2",
						Command: []string{"/bin/sh", "-c"},
						Args: []string{"set -o pipefail; mysqldump -u " + m.Spec.Username + " -h " + m.Spec.Host + " -P " + m.Spec.Port + " -p$MYSQL_PASSWORD " +
							m.Spec.Database + "| pigz -9 -p 4 > " + m.Spec.Database + ".sql.gz"},
						//Command: []string{"sleep", "60000"},
						Env: []corev1.EnvVar{{
							Name: "MYSQL_PASSWORD",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: m.Spec.SecretRef.Secret,
									},
									Key: "ROOT_PASSWORD",
								},
							},
						}},
						TerminationMessagePath:   "/tmp/error.log",
						TerminationMessagePolicy: "File",
					}},
				},
			},
		},
	}
	ctrl.SetControllerReference(m, job, r.Scheme)
	return job
}

// updateStatus sets the given status for the cr
func updateStatus(r *MysqlBackupReconciler, m *m1kcloudv1alpha1.MysqlBackup, status string) bool {
	ctx := context.Background()
	m.Status.BackupStatus = status
	err := r.Status().Update(ctx, m)
	if err != nil {
		log.Error(err, "Failed to update cr status")
		return false
	}
	return true
}

// getJobStatus returns job names, status of the array of jobs passed in
func getjobStatus(jobs []batchv1.Job) map[string]batchv1.JobStatus {
	var jobStatusMap map[string]batchv1.JobStatus
	jobStatusMap = make(map[string]batchv1.JobStatus)
	for _, job := range jobs {
		jobStatusMap[job.Name] = job.Status
	}
	return jobStatusMap
}

// contains is a simple function to check for a string in a slice
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// return the label to filter backupjobs
func joblabels(name string, cluster string, database string) map[string]string {
	return map[string]string{"MysqlBackup": name, "cluster": cluster, "database": database}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MysqlBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&m1kcloudv1alpha1.MysqlBackup{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
