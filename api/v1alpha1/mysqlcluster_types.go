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

// MysqlClusterSpec defines the desired state of MysqlCluster
type MysqlClusterSpec struct {
	Host      string             `json:"host"`
	Port      int                `json:"port,omitempty"`
	Username  string             `json:"username"`
	SecretRef MysqlSecretRefSpec `json:"secretRef"`
}

// MysqlSecretRefSpec defines the MysqlSecret ref
type MysqlSecretRefSpec struct {
	Secret    string `json:"secret"`
	Namespace string `json:"namespace"`
}

// MysqlClusterStatus defines the observed state of MysqlCluster
type MysqlClusterStatus struct {
	ClusterStatus string `json:"clusterStatus"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MysqlCluster is the Schema for the mysqlclusters API
type MysqlCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MysqlClusterSpec   `json:"spec,omitempty"`
	Status MysqlClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MysqlClusterList contains a list of MysqlCluster
type MysqlClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MysqlCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MysqlCluster{}, &MysqlClusterList{})
}
