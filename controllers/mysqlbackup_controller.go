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
	"k8s.io/apimachinery/pkg/types"
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

// Reconcile MysqlBackup CRD
func (r *MysqlBackupReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := log.WithValues("mysqlbackup", req.NamespacedName)

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
	if status == "" {
		status = "newBackup"
	}

	// List the jobs for this mysqlbackup crd
	jobList := &batchv1.JobList{}
	listOpts := []client.ListOption{
		client.InNamespace(mysqlbackup.Namespace),
		client.MatchingLabels(joblabels(mysqlbackup.Name)),
	}
	if err = r.List(ctx, jobList, listOpts...); err != nil {
		log.Error(err, "Failed to get Job list")
		return ctrl.Result{}, err
	}

	/*
		// Update mysqlbackup crd status with job results
		if len(jobList.Items) > 0 {
			// retrieve job status for all jobs
			log.Info("whyyyyyyy")
			jobStatus := getjobStatus(jobList.Items)
			for obj, objStatus := range jobStatus {
				if objStatus.Succeeded == 1 {
					mysqlbackup.Status.SuccessfulJobs = append(mysqlbackup.Status.SuccessfulJobs, obj)
					mysqlbackup.Status.SuccessfulBackupCount = mysqlbackup.Status.SuccessfulBackupCount + 1
					mysqlbackup.Status.BackupStatus = "readyBackup"
					log.Info("Backup Job succeded")
				} else if objStatus.Failed == 1 {
					mysqlbackup.Status.FailedJobs = append(mysqlbackup.Status.FailedJobs, obj)
					mysqlbackup.Status.BackupStatus = "failedBackup"
					log.Info("Backup Job failed")
				} else if objStatus.Active == 1 {
					log.Info("Backup Job still running")
				}
				err := r.Status().Update(ctx, mysqlbackup)
				if err != nil {
					log.Error(err, "Failed to update mysqlbackup BackupStatus")
					return ctrl.Result{}, err
				}
			}
		}
	*/

	// Check mysqlbackup status and act accordingly
	switch status {

	case "newBackup":
		log.Info("Start newBackup phase")
		// Set creatingBackup status and requeue if
		mysqlbackup.Status.BackupStatus = "creatingBackup"
		//mysqlbackup.Status.SuccessfulJobs = nil
		//mysqlbackup.Status.FailedJobs = nil
		err := r.Status().Update(ctx, mysqlbackup)
		if err != nil {
			log.Error(err, "Failed to update mysqlbackup BackupStatus")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil

	case "creatingBackup":
		log.Info("Start creatingBackup phase")
		// Check if the Job already exists, if not create a new one
		found := &batchv1.Job{}
		err = r.Get(ctx, types.NamespacedName{Name: mysqlbackup.Name, Namespace: mysqlbackup.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			// Define a new Job
			job := r.mysqlBackupJob(mysqlbackup)
			log.Info("Creating a new Job")
			err = r.Create(ctx, job)
			if err != nil {
				log.Error(err, "Failed to create new Job")
				return ctrl.Result{}, err
			}
			// Job created successfully - return and requeue
			log.Info("Job Created Successfully, return and requeue")
			return ctrl.Result{Requeue: true}, nil
		} else if err != nil {
			log.Error(err, "Failed to get Job")
			return ctrl.Result{}, err
		} else {
			log.Info("Job already present, nothing to do")
		}

	case "failedBackup":
		log.Info("Start failedBackup phase")

	case "readyBackup":
		log.Info("Start readyBackup phase")

	default:
		log.Info("Start unknown backup state phase")
	}

	return ctrl.Result{}, nil
}

// mysqlBackupJob returns a mysql backup job object
func (r *MysqlBackupReconciler) mysqlBackupJob(m *m1kcloudv1alpha1.MysqlBackup) *batchv1.Job {
	ls := joblabels(m.Name)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "Never",
					Containers: []corev1.Container{{
						Name:    "mysqldump",
						Image:   "quay.io/ironashram/test-alpine:v0.0.2",
						Command: []string{"/bin/sh", "-c"},
						Args: []string{"mysqldump -u " + m.Spec.Username + " -h " + m.Spec.Host + " -P " + m.Spec.Port + " -p$MYSQL_PASSWORD " +
							m.Spec.DatabasesToBackup[0] + "| pigz -9 -p 4 > " + m.Spec.DatabasesToBackup[0] + ".sql.gz 2> /tmp/error.log"},
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
	// Set mysqlBackupJob instance as the owner and controller
	ctrl.SetControllerReference(m, job, r.Scheme)
	return job
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

// return the label to filter backupjobs
func joblabels(name string) map[string]string {
	return map[string]string{"app": "mysqlBackup", "mysqlBackup_crd": name}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MysqlBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&m1kcloudv1alpha1.MysqlBackup{}).
		Complete(r)
}
