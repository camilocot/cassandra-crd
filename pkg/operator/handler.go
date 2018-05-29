package operator

import (
	"fmt"

	"github.com/camilocot/cassandra-crd/pkg/log"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	cassandrav1alpha1 "github.com/camilocot/cassandra-crd/pkg/apis/cassandra/v1alpha1"
	ccsvc "github.com/camilocot/cassandra-crd/pkg/operator/service"
)

// Handler  is the cassandra cluster handler that will handle the
// events received from kubernetes.
type handler struct {
	k8sCli kubernetes.Interface
	ccSvc  ccsvc.CassandraClusterClient
	logger log.Logger
}

// newHandler returns a new handler.
func newHandler(k8sCli kubernetes.Interface, ccSvc ccsvc.CassandraClusterClient, logger log.Logger) *handler {
	return &handler{
		k8sCli: k8sCli,
		ccSvc:  ccSvc,
		logger: logger,
	}
}

func (h *handler) Add(obj runtime.Object) error {
	cc, ok := obj.(*cassandrav1alpha1.CassandraCluster)
	if !ok {
		return fmt.Errorf("%v is not a cassandra cluster object", obj.GetObjectKind())
	}

	if err := h.Ensure(cc); err != nil {
		return err
	}

	return nil
}

func (h *handler) Delete(name string) error {

	fmt.Println(name)
	return nil
}

func (h *handler) Ensure(cc *cassandrav1alpha1.CassandraCluster) error {
	if err := h.ccSvc.EnsureStatefulset(cc); err != nil {
		return err
	}

	return nil
}
