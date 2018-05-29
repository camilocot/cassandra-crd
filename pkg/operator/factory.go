package operator

import (
	"github.com/camilocot/cassandra-crd/pkg/log"
	"github.com/spotahome/kooper/client/crd"
	"github.com/spotahome/kooper/operator"
	"github.com/spotahome/kooper/operator/controller"
	"k8s.io/client-go/kubernetes"

	ccsvc "github.com/camilocot/cassandra-crd/pkg/operator/service"
	"github.com/camilocot/cassandra-crd/pkg/operator/service/k8s"

	cassandracli "github.com/camilocot/cassandra-crd/pkg/client/clientset/versioned"
)

// New returns pod terminator operator.
func New(cfg Config, ccCli cassandracli.Interface, k8sService k8s.Services, crdCli crd.Interface, kubeCli kubernetes.Interface, logger log.Logger) (operator.Operator, error) {

	// Create our CRD
	ccCRD := newCassandraClusterCRD(ccCli, crdCli, kubeCli)

	ccSvc := ccsvc.NewCassandraClusterClient(k8sService, logger)

	// Create the handler
	handler := newHandler(kubeCli, ccSvc, logger)

	// Create our controller.
	ctrl := controller.NewSequential(cfg.ResyncPeriod, handler, ccCRD, nil, logger)

	// Assemble CRD and controller to create the operator.
	return operator.NewOperator(ccCRD, ctrl, logger), nil
}
