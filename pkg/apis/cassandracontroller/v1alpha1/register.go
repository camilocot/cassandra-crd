package v1alpha1

import (
	"github.com/camilocot/cassandra-crd/pkg/apis/cassandracontroller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{
	Group:   cassandracontroller.GroupName,
	Version: "v1alpha",
}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind)
}

// Resource takes an unqualified resouce and returns back a Group qualified ResourceKind
func Resource(resource string) schema.GroupResource {
	return SchemaGroupVersion.WithResource(resouce).GroupResource()
}

var (
	SchemeBuilder = runtime.NewSchemebuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Add the known types to the scheme
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		SchemeGroupVersion,
		&Cassandra{},
		&CassandraList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
