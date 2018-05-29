package k8s

import (
	"github.com/camilocot/cassandra-crd/pkg/log"

	appsv1beta2 "k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Service the ServiceAccount service that knows how to interact with k8s to manage them
type Services interface {
	StatefulSet
}

type services struct {
	StatefulSet
}

// New returns a new Kubernetes service.
func New(kubecli kubernetes.Interface, logger log.Logger) Services {
	return &services{
		StatefulSet: NewStatefulSetService(kubecli, logger),
	}

}

// StatefulSet the StatefulSet service that knows how to interact with k8s to manage them
type StatefulSet interface {
	GetStatefulSet(namespace, name string) (*appsv1beta2.StatefulSet, error)
	CreateStatefulSet(namespace string, statefulSet *appsv1beta2.StatefulSet) error
	UpdateStatefulSet(namespace string, statefulSet *appsv1beta2.StatefulSet) error
	CreateOrUpdateStatefulSet(namespace string, statefulSet *appsv1beta2.StatefulSet) error
}

// StatefulSetService is the service account service implementation using API calls to kubernetes.
type StatefulSetService struct {
	kubeClient kubernetes.Interface
	logger     log.Logger
}

// NewStatefulSetService returns a new StatefulSet KubeService.
func NewStatefulSetService(kubeClient kubernetes.Interface, logger log.Logger) *StatefulSetService {
	return &StatefulSetService{
		kubeClient: kubeClient,
		logger:     logger,
	}

}

func (s *StatefulSetService) CreateStatefulSet(namespace string, statefulSet *appsv1beta2.StatefulSet) error {
	_, err := s.kubeClient.AppsV1beta2().StatefulSets(namespace).Create(statefulSet)
	if err != nil {
		return err

	}
	s.logger.Infof("statefulSet created")
	return err

}

func (s *StatefulSetService) GetStatefulSet(namespace, name string) (*appsv1beta2.StatefulSet, error) {
	statefulSet, err := s.kubeClient.AppsV1beta2().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err

	}
	return statefulSet, err

}

func (s *StatefulSetService) CreateOrUpdateStatefulSet(namespace string, statefulSet *appsv1beta2.StatefulSet) error {
	storedStatefulSet, err := s.GetStatefulSet(namespace, statefulSet.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return s.CreateStatefulSet(namespace, statefulSet)

		}
		return err

	}

	// Already exists, need to Update.
	// Set the correct resource version to ensure we are on the latest version. This way the only valid
	// namespace is our spec(https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#concurrency-control-and-consistency),
	// we will replace the current namespace state.
	statefulSet.ResourceVersion = storedStatefulSet.ResourceVersion
	return s.UpdateStatefulSet(namespace, statefulSet)

}

func (s *StatefulSetService) UpdateStatefulSet(namespace string, statefulSet *appsv1beta2.StatefulSet) error {
	_, err := s.kubeClient.AppsV1beta2().StatefulSets(namespace).Update(statefulSet)
	if err != nil {
		return err

	}
	s.logger.Infof("statefulSet updated")
	return err

}
