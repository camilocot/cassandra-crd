package service

import (
	"github.com/camilocot/cassandra-crd/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cassandrav1alpha1 "github.com/camilocot/cassandra-crd/pkg/apis/cassandra/v1alpha1"
	"github.com/camilocot/cassandra-crd/pkg/operator/service/k8s"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type CassandraClusterClient interface {
	EnsureStatefulset(*cassandrav1alpha1.CassandraCluster) error
}

type CassandraClusterKubeClient struct {
	K8SService k8s.Services
	logger     log.Logger
}

// NewRedisFailoverKubeClient creates a new RedisFailoverKubeClient
func NewCassandraClusterClient(k8sService k8s.Services, logger log.Logger) *CassandraClusterKubeClient {
	return &CassandraClusterKubeClient{
		K8SService: k8sService,
		logger:     logger,
	}
}

// EnsureStatefulset makes sure the cassandra statefulset exists in the desired state
func (r *CassandraClusterKubeClient) EnsureStatefulset(cc *cassandrav1alpha1.CassandraCluster) error {
	ss := r.generateCassandraStatefulSet(cc)
	return r.K8SService.CreateOrUpdateStatefulSet(cc.Namespace, ss)
}
func (r *CassandraClusterKubeClient) generateCassandraStatefulSet(cc *cassandrav1alpha1.CassandraCluster) *appsv1beta2.StatefulSet {
	labels := map[string]string{
		"app":        "cassandra",
		"controller": cc.Name,
	}
	return &appsv1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cc.Spec.StatefulSetName,
			Namespace: cc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cc, schema.GroupVersionKind{
					Group:   cassandrav1alpha1.SchemeGroupVersion.Group,
					Version: cassandrav1alpha1.SchemeGroupVersion.Version,
					Kind:    "CassandraCluster",
				}),
			},
		},
		Spec: appsv1beta2.StatefulSetSpec{
			ServiceName: cc.Spec.StatefulSetName + "-unready",
			Replicas:    cc.Spec.Replicas,
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
									Value: cc.Spec.StatefulSetName + "-0." + cc.Spec.StatefulSetName + "-unready." + cc.Namespace + ".svc.cluster.local",
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
