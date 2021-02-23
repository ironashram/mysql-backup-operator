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
	//log := ctrl.Log.WithValues("mysqlbackup", req.NamespacedName)

	// fetch mysqlbackup
	mysqlbackup := &m1kcloudv1alpha1.MysqlBackup{}
	err := r.Get(ctx, req.NamespacedName, mysqlbackup)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("MysqlBackup Controller", "MysqlBackup.Namespace", mysqlbackup.Namespace, "MysqlBackup.Name",
				mysqlbackup.Name, "msg", "MysqlBackup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "MysqlBackup Controller", "MysqlBackup.Namespace", mysqlbackup.Namespace, "MysqlBackup.Name",
			mysqlbackup.Name, "msg", "Failed to get MysqlBackup")
		return ctrl.Result{}, err
	}

	// Check mysqlbackup status and act accordingly
	status := mysqlbackup.Spec.InitState
	switch status {
	case "newBackup":
		log.Info("MysqlBackup Controller", "MysqlBackup.initStatus", status, "MysqlBackup.Namespace", mysqlbackup.Namespace,
			"MysqlBackup.Name", mysqlbackup.Name, "msg", "Check Backup Job existence")
		// Check if the Job already exists, if not create a new one
		found := &batchv1.Job{}
		err = r.Get(ctx, types.NamespacedName{Name: mysqlbackup.Name, Namespace: mysqlbackup.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			// Define a new Job
			job := r.mysqlBackupJob(mysqlbackup)
			log.Info("MysqlBackup Controller", "Job.Namespace", job.Namespace, "Job.Name", job.Name, "msg", "Creating a new Job")
			err = r.Create(ctx, job)
			if err != nil {
				log.Error(err, "MysqlBackup Controller", "Job.Namespace", job.Namespace, "Job.Name", job.Name, "msg", "Failed to create new Job")
				return ctrl.Result{}, err
			}
			// Job created successfully - return and requeue
			log.Info("MysqlBackup Controller", "Job.Namespace", job.Namespace, "Job.Name", job.Name, "msg", "Return and Requeue")
			return ctrl.Result{Requeue: true}, nil
		} else if err != nil {
			log.Error(err, "MysqlBackup Controller", "MysqlBackup.Namespace", mysqlbackup.Namespace,
				"MysqlBackup.Name", mysqlbackup.Name, "msg", "Failed to get Job")
			return ctrl.Result{}, err
		} else {
			log.Info("MysqlBackup Controller", "MysqlBackup.initStatus", status, "MysqlBackup.Namespace", mysqlbackup.Namespace,
				"MysqlBackup.Name", mysqlbackup.Name, "msg", "Job already present, nothing to do")
		}
	case "creatingBackup":
		log.Info("MysqlBackup Controller", "MysqlBackup.initStatus", status, "MysqlBackup.Namespace", mysqlbackup.Namespace,
			"MysqlBackup.Name", mysqlbackup.Name, "msg", "")
	case "failedBackup":
		log.Info("MysqlBackup Controller", "MysqlBackup.initStatus", status, "MysqlBackup.Namespace", mysqlbackup.Namespace,
			"MysqlBackup.Name", mysqlbackup.Name, "msg", "")
	case "readyBackup":
		log.Info("MysqlBackup Controller", "MysqlBackup.initStatus", status, "MysqlBackup.Namespace", mysqlbackup.Namespace,
			"MysqlBackup.Name", mysqlbackup.Name, "msg", "")
	default:
		log.Info("MysqlBackup Controller", "MysqlBackup.initStatus", status, "MysqlBackup.Namespace", mysqlbackup.Namespace,
			"MysqlBackup.Name", mysqlbackup.Name, "msg", "")
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
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: "Never",
					Containers: []corev1.Container{{
						Name:    "mysql-backupjob-" + m.Name,
						Image:   "quay.io/ironashram/test-alpine:v0.0.2",
						Command: []string{"/bin/sh", "-c"},
						Args: []string{m.Spec.BackupType + " -u " + m.Spec.Username + " -h " + m.Spec.Host + " -P " + m.Spec.Port + " -p$MYSQL_PASSWORD " +
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
