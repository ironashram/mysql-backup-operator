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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MysqlBackupSpec defines the desired state of MysqlBackup
type MysqlBackupSpec struct {
	Host              string         `json:"host"`
	Port              int            `json:"port,omitempty"`
	Username          string         `json:"username"`
	BackupType        string         `json:"backupType"`
	ClusterRef        ClusterRefSpec `json:"clusterRef"`
	StorageRef        StorageRefSpec `json:"storageRef"`
	DatabasesToBackup []string       `json:"databasesToBackup"`
	SecretRef         SecretRefSpec  `json:"secretRef"`
}

// ClusterRefSpec defines the ClusterRef
type ClusterRefSpec struct {
	ClusterName   string `json:"clusterName"`
	ClusterStatus string `json:"clusterStatus"`
}

// StorageRefSpec defines the StorageRef
type StorageRefSpec struct {
	StorageBackend string `json:"storageBackend"`
	ClusterStatus  string `json:"clusterstatus"`
}

// SecretRefSpec defines the SecretRef
type SecretRefSpec struct {
	Secret    string `json:"secret"`
	Namespace string `json:"namespace"`
}

// MysqlBackupStatusSpec defines the backup status
type MysqlBackupStatusSpec struct {
	BackupStatus              string   `json:"backupStatus"`
	BackupJobStatus           string   `json:"backupJobStatus"`
	SuccessfulBackupLocations []string `json:"successfulBackupLocations"`
}

// MysqlBackupStatus defines the observed state of MysqlBackup
type MysqlBackupStatus struct {
	BackupStatus MysqlBackupStatusSpec `json:"backupStatus"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MysqlBackup is the Schema for the mysqlbackups API
type MysqlBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MysqlBackupSpec   `json:"spec,omitempty"`
	Status MysqlBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MysqlBackupList contains a list of MysqlBackup
type MysqlBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MysqlBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MysqlBackup{}, &MysqlBackupList{})
}
