/*
Copyright 2018 The cassandra-crd Authors.

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

package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	cassandraapi "github.com/camilocot/cassandra-crd/pkg/apis/cassandra/v1alpha1"
	clientset "github.com/camilocot/cassandra-crd/pkg/client/clientset/versioned"
	cassandrascheme "github.com/camilocot/cassandra-crd/pkg/client/clientset/versioned/scheme"
	informers "github.com/camilocot/cassandra-crd/pkg/client/informers/externalversions"
	listers "github.com/camilocot/cassandra-crd/pkg/client/listers/cassandra/v1alpha1"
)

const controllerAgentName = "cassandra-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a CassandraCluster is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a CassandraCluster fails
	// to sync due to a StatefulSet of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a StatefulSet already existing
	MessageResourceExists = "Resource %q already exists and is not managed by CassandraCluster"
	// MessageResourceSynced is the message used for an Event fired when a CassandraCluster
	// is synced successfully
	MessageResourceSynced = "CassandraCluster synced successfully"
)

// Controller is the controller implementation for CassandraCluster resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// cassandraclientset is a clientset for our own API group
	cassandraclientset clientset.Interface

	statefulsetsLister      appslisters.StatefulSetLister
	statefulsetsSynced      cache.InformerSynced
	cassandraclustersLister listers.CassandraClusterLister
	cassandraclustersSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new cassandra controller
func NewController(
	kubeclientset kubernetes.Interface,
	cassandraclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	cassandraInformerFactory informers.SharedInformerFactory) *Controller {

	// obtain references to shared index informers for the StatefulSet and CassandraCluster
	// types.
	statefulsetInformer := kubeInformerFactory.Apps().V1().StatefulSets()
	cassandraclusterInformer := cassandraInformerFactory.Cassandra().V1alpha1().CassandraClusters()

	// Create event broadcaster
	// Add cassandra-controller types to the default Kubernetes Scheme so Events can be
	// logged for cassandra-controller types.
	cassandrascheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:           kubeclientset,
		cassandraclientset:      cassandraclientset,
		statefulsetsLister:      statefulsetInformer.Lister(),
		statefulsetsSynced:      statefulsetInformer.Informer().HasSynced,
		cassandraclustersLister: cassandraclusterInformer.Lister(),
		cassandraclustersSynced: cassandraclusterInformer.Informer().HasSynced,
		workqueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "CassandraClusters"),
		recorder:                recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when CassandraCluster resources change
	cassandraclusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueCassandraCluster,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueCassandraCluster(new)
		},
	})
	// Set up an event handler for when StatefulSet resources change. This
	// handler will lookup the owner of the given StatefulSet, and if it is
	// owned by a CassandraCluster resource will enqueue that CassandraCluster resource for
	// processing. This way, we don't need to implement custom logic for
	// handling StatefulSet resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	statefulsetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newStSet := new.(*appsv1.StatefulSet)
			oldStSet := old.(*appsv1.StatefulSet)
			if newStSet.ResourceVersion == oldStSet.ResourceVersion {
				// Periodic resync will send update events for all known StatefulSets.
				// Two different versions of the same StatefulSet will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting CassandraCluster controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.statefulsetsSynced, c.cassandraclustersSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process CassandraCluster resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// CassandraCluster resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the CassandraCluster resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CassandraCluster resource with this namespace/name
	cassandracluster, err := c.cassandraclustersLister.CassandraClusters(namespace).Get(name)
	if err != nil {
		// The CassandraCluster resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("cassandracluster '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	statefulsetName := cassandracluster.Spec.StatefulSetName
	if statefulsetName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: statefulset name must be specified", key))
		return nil
	}

	// Get the headless service with the name specified in CassandraCluster.spec
	_, err = c.kubeclientset.CoreV1().Services(cassandracluster.Namespace).Get(statefulsetName+"-unready", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = c.kubeclientset.CoreV1().Services(cassandracluster.Namespace).Create(newHeadLessServiceUnready(cassandracluster))
	}

	if err != nil {
		return err
	}
	// Get the headless service with the name specified in CassandraCluster.spec
	_, err = c.kubeclientset.CoreV1().Services(cassandracluster.Namespace).Get(statefulsetName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = c.kubeclientset.CoreV1().Services(cassandracluster.Namespace).Create(newHeadLessService(cassandracluster))
	}

	if err != nil {
		return err
	}

	// Get the statefulset with the name specified in CassandraCluster.spec
	statefulset, err := c.statefulsetsLister.StatefulSets(cassandracluster.Namespace).Get(statefulsetName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		statefulset, err = c.kubeclientset.AppsV1().StatefulSets(cassandracluster.Namespace).Create(newStatefulSet(cassandracluster))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the StatefulSet is not controlled by this CassandraCluster resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(statefulset, cassandracluster) {
		msg := fmt.Sprintf(MessageResourceExists, statefulset.Name)
		c.recorder.Event(cassandracluster, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// If this number of the replicas on the CassandraCluster resource is specified, and the
	// number does not equal the current desired replicas on the StatefulSet, we
	// should update the StatefulSet resource.
	if cassandracluster.Spec.Replicas != nil && *cassandracluster.Spec.Replicas != *statefulset.Spec.Replicas {
		glog.V(4).Infof("CassandraCluster %s replicas: %d, statefulset replicas: %d", name, *cassandracluster.Spec.Replicas, *statefulset.Spec.Replicas)
		statefulset, err = c.kubeclientset.AppsV1().StatefulSets(cassandracluster.Namespace).Update(newStatefulSet(cassandracluster))
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. THis could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the CassandraCluster resource to reflect the
	// current state of the world
	err = c.updateCassandraClusterStatus(cassandracluster, statefulset)
	if err != nil {
		return err
	}

	c.recorder.Event(cassandracluster, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateCassandraClusterStatus(cassandracluster *cassandraapi.CassandraCluster, statefulset *appsv1.StatefulSet) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	cassandraclusterCopy := cassandracluster.DeepCopy()
	cassandraclusterCopy.Status.CurrentReplicas = statefulset.Status.CurrentReplicas
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the CassandraCluster resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.cassandraclientset.CassandraV1alpha1().CassandraClusters(cassandracluster.Namespace).Update(cassandraclusterCopy)
	return err
}

// enqueueCassandraCluster takes a CassandraCluster resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than CassandraCluster.
func (c *Controller) enqueueCassandraCluster(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the CassandraCluster resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that CassandraCluster resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a CassandraCluster, we should not do anything more
		// with it.
		if ownerRef.Kind != "CassandraCluster" {
			return
		}

		cassandracluster, err := c.cassandraclustersLister.CassandraClusters(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of cassandracluster '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueCassandraCluster(cassandracluster)
		return
	}
}

// newStatefulSet creates a new StatefulSet for a CassandraCluster resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the CassandraCluster resource that 'owns' it.
func newStatefulSet(cassandracluster *cassandraapi.CassandraCluster) *appsv1.StatefulSet {
	labels := map[string]string{
		"app":        "cassandra",
		"controller": cassandracluster.Name,
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cassandracluster.Spec.StatefulSetName,
			Namespace: cassandracluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cassandracluster, schema.GroupVersionKind{
					Group:   cassandraapi.SchemeGroupVersion.Group,
					Version: cassandraapi.SchemeGroupVersion.Version,
					Kind:    "CassandraCluster",
				}),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: cassandracluster.Spec.StatefulSetName + "-unready",
			Replicas:    cassandracluster.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "cassandra",
							Image: "gcr.io/google-samples/cassandra:v13",
							Env: []corev1.EnvVar{
								{
									Name:  "CASSANDRA_SEEDS",
									Value: cassandracluster.Spec.StatefulSetName + "-0." + cassandracluster.Spec.StatefulSetName + "-unready." + cassandracluster.Namespace + ".svc.cluster.local",
								},
								{
									Name:  "MAX_HEAP_SIZE",
									Value: "512M",
								},
								{
									Name:  "HEAP_NEWSIZE",
									Value: "100M",
								},
								{
									Name: "POD_IP",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "cql",
									ContainerPort: 9042,
								},
								{
									Name:          "intra-node",
									ContainerPort: 7001,
								},
								{
									Name:          "jmx",
									ContainerPort: 7099,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"IPC_LOCK"},
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/bash", "-c", "/ready-probe.sh"},
									},
								},
								InitialDelaySeconds: 15,
								TimeoutSeconds:      5,
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/sh", "-c", "nodetool", "drain"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func newHeadLessServiceUnready(cassandracluster *cassandraapi.CassandraCluster) *corev1.Service {
	labels := map[string]string{
		"app":        "cassandra",
		"controller": cassandracluster.Name,
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cassandracluster.Spec.StatefulSetName + "-unready",
			Labels:    labels,
			Namespace: cassandracluster.Namespace,
			// it will return IPs even of the unready pods. Bootstraping a new cluster need it
			Annotations: map[string]string{
				"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cassandracluster, schema.GroupVersionKind{
					Group:   cassandraapi.SchemeGroupVersion.Group,
					Version: cassandraapi.SchemeGroupVersion.Version,
					Kind:    "CassandraCluster",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "cql",
					Port:       9042,
					TargetPort: intstr.FromInt(9042),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector:  labels,
			ClusterIP: "None",
			Type:      corev1.ServiceTypeClusterIP,
		},
	}
}

// newHeadLessService creates a new headless Service for a CassandraCluster resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the CassandraCluster resource that 'owns' it.
func newHeadLessService(cassandracluster *cassandraapi.CassandraCluster) *corev1.Service {
	labels := map[string]string{
		"app":        "cassandra",
		"controller": cassandracluster.Name,
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cassandracluster.Spec.StatefulSetName,
			Labels:    labels,
			Namespace: cassandracluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cassandracluster, schema.GroupVersionKind{
					Group:   cassandraapi.SchemeGroupVersion.Group,
					Version: cassandraapi.SchemeGroupVersion.Version,
					Kind:    "CassandraCluster",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "cql",
					Port:       9042,
					TargetPort: intstr.FromInt(9042),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector:  labels,
			ClusterIP: "None",
			Type:      corev1.ServiceTypeClusterIP,
		},
	}
}
