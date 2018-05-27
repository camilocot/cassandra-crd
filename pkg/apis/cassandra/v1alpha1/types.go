package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CassandraCluster is a specification for a CassandraCluster resource
type CassandraCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CassandraClusterSpec   `json:"spec"`
	Status CassandraClusterStatus `json:"status"`
}

// CassandraClusterSpec is the spec for a CassandraCluster resource
type CassandraClusterSpec struct {
	StatefulSetName string `json:"statefulsetName"`
	Replicas        *int32 `json:"replicas"`
}

// CassandraClusterStatus is the status for a CassandraCluster resource
type CassandraClusterStatus struct {
	CurrentReplicas int32 `json:"currentReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CassandraClusterList is a list of CassandraCluster resources
type CassandraClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CassandraCluster `json:"items"`
}
