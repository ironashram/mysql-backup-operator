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

	m1kcloudv1alpha1 "github.com/ironashram/mysql-backup-operator/api/v1alpha1"
)

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
	log := r.Log.WithValues("mysqlbackup", req.NamespacedName)

	// fetch mysqlbackup
	mysqlbackup := &m1kcloudv1alpha1.MysqlBackup{}
	err := r.Get(ctx, req.NamespacedName, mysqlbackup)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("MysqlBackup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get MysqlBackup")
		return ctrl.Result{}, err
	}

	// Check mysqlbackup status and act accordingly
	status := mysqlbackup.Spec.InitState
	switch status {
	case "newBackup":
		log.Info("MysqlBackup status is newBackup")
	case "creatingBackup":
		log.Info("MysqlBackup status is creatingBackup")
	case "failedBackup":
		log.Info("MysqlBackup status is failedBackup")
	case "readyBackup":
		log.Info("MysqlBackup status is readyBackup")
	default:
		log.Info("MysqlBackup status is not in recognized state")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MysqlBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&m1kcloudv1alpha1.MysqlBackup{}).
		Complete(r)
}
