package operator

import (
	"github.com/spotahome/kooper/client/crd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	cassandrav1alpha1 "github.com/camilocot/cassandra-crd/pkg/apis/cassandra/v1alpha1"
	cassandracli "github.com/camilocot/cassandra-crd/pkg/client/clientset/versioned"
)

// cassandraClusterCRD is the crd cassandra cluster
type cassandraClusterCRD struct {
	crdCli  crd.Interface
	kubeCli kubernetes.Interface
	ccCli   cassandracli.Interface
}

func newCassandraClusterCRD(ccCli cassandracli.Interface, crdCli crd.Interface, kubeCli kubernetes.Interface) *cassandraClusterCRD {
	return &cassandraClusterCRD{
		crdCli:  crdCli,
		ccCli:   ccCli,
		kubeCli: kubeCli,
	}
}

// Initialize satisfies resource.crd interface.
func (cc *cassandraClusterCRD) Initialize() error {
	crd := crd.Conf{
		Kind:       cassandrav1alpha1.CCKind,
		NamePlural: cassandrav1alpha1.CCNamePlural,
		Group:      cassandrav1alpha1.SchemeGroupVersion.Group,
		Version:    cassandrav1alpha1.SchemeGroupVersion.Version,
		Scope:      cassandrav1alpha1.CCScope,
	}

	return cc.crdCli.EnsurePresent(crd)
}

// GetListerWatcher satisfies resource.crd interface (and retrieve.Retriever).
func (cc *cassandraClusterCRD) GetListerWatcher() cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return cc.ccCli.CassandraV1alpha1().CassandraClusters("default").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return cc.ccCli.CassandraV1alpha1().CassandraClusters("default").Watch(options)
		},
	}
}

// GetObject satisfies resource.crd interface (and retrieve.Retriever).
func (cc *cassandraClusterCRD) GetObject() runtime.Object {
	return &cassandrav1alpha1.CassandraCluster{}
}
