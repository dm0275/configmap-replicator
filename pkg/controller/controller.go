package controller

import (
	"context"
	"github.com/dm0275/configmap-replicator-operator/utils"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"log"
	"strconv"
	"time"
)

var logger = log.Default()

// ConfigMapReplicatorController is responsible for replicating ConfigMaps.
type ConfigMapReplicatorController struct {
	clientset              *kubernetes.Clientset
	ReconciliationInterval *time.Duration
	ExcludedNamespaces     *[]string
	AllowedNamespaces      *[]string
}

// NewConfigMapReplicatorController creates a new instance of the ConfigMapReplicatorController.
func NewConfigMapReplicatorController(config *rest.Config, reconciliationInterval string, excludedNamespaces, allowedNamespaces []string) *ConfigMapReplicatorController {
	interval, err := time.ParseDuration(reconciliationInterval)
	if err != nil {
		logger.Fatalf("Invalid reconciliation interval %s: %v\n", reconciliationInterval, err)
	}

	// Create a Kubernetes clientset.
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatalf("Error creating Kubernetes clientset: %v\n", err)
	}

	controller := &ConfigMapReplicatorController{
		clientset:              clientset,
		ReconciliationInterval: &interval,
		ExcludedNamespaces:     &excludedNamespaces,
		AllowedNamespaces:      &allowedNamespaces,
	}

	// Validate Controller configuration
	controller.configureNamespaces()

	return controller
}

func (c *ConfigMapReplicatorController) configureNamespaces() {
	if utils.SlicesOverlap(*c.AllowedNamespaces, *c.ExcludedNamespaces) {
		logger.Fatalf("ERROR: Cannot have overlaps between allowedNamespaces and excludedNamespaces")
	}
}

// Run starts the controller and watches for ConfigMap changes.
func (c *ConfigMapReplicatorController) replicateEnabled(configMap *v1.ConfigMap) bool {
	replicationAllowed, ok := configMap.Annotations["replication-allowed"]
	if !ok {
		return false
	}

	replicationAllowedBool, err := strconv.ParseBool(replicationAllowed)
	if err != nil {
		return false
	}

	return replicationAllowedBool
}

// Replicate the given ConfigMap to all namespaces
func (c *ConfigMapReplicatorController) addConfigMapAcrossNamespaces(ctx context.Context, configMap *v1.ConfigMap) {
	if c.replicateEnabled(configMap) {
		if len(*c.AllowedNamespaces) > 0 {
			for _, ns := range *c.AllowedNamespaces {
				// Create a new ConfigMap
				c.createConfigMap(ctx, configMap, ns)
			}
		} else {
			namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				logger.Printf("Error listing namespaces: %v", err)
				return
			}

			for _, ns := range namespaces.Items {
				if configMap.Namespace == ns.Name {
					logger.Printf("ConfigMap %s in the %s namespace is a source ConfigMap", configMap.Name, configMap.Namespace)
					continue
				} else if utils.ListContains(*c.ExcludedNamespaces, ns.Name) {
					logger.Printf("Namespace %s is an excluded Namespace. Not replicating ConfigMap %s to Namespace %s.", ns.Name, configMap.Name, ns.Name)
					continue
				} else {
					// Create a new ConfigMap in each namespace
					c.createConfigMap(ctx, configMap, ns.Name)
				}
			}
		}
	}
}

func (c *ConfigMapReplicatorController) createConfigMap(ctx context.Context, configMap *v1.ConfigMap, ns string) {
	// Create a new ConfigMap in each namespace
	newConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMap.Name,
			Namespace: ns,
			Annotations: map[string]string{
				"replicated-from": configMap.Namespace + "_" + configMap.Name,
			},
		},
		Data: configMap.Data,
	}

	_, err := c.clientset.CoreV1().ConfigMaps(ns).Create(ctx, newConfigMap, metav1.CreateOptions{})
	if err != nil {
		logger.Printf("Error replicating ConfigMap to namespace %s: %v", ns, err)
	} else {
		logger.Printf("Replicated ConfigMap %s to namespace %s", configMap.Name, ns)
	}
}

func (c *ConfigMapReplicatorController) updateConfigMap(ctx context.Context, configMap *v1.ConfigMap, ns string) {
	updatedConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMap.Name,
			Namespace: ns,
			Annotations: map[string]string{
				"replicated-from": configMap.Namespace + "_" + configMap.Name,
			},
		},
		Data: configMap.Data,
	}

	_, err := c.clientset.CoreV1().ConfigMaps(ns).Get(ctx, updatedConfigMap.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err = c.clientset.CoreV1().ConfigMaps(ns).Create(ctx, updatedConfigMap, metav1.CreateOptions{})
			if err != nil {
				logger.Printf("Error replicating ConfigMap to namespace %s: %v", ns, err)
			} else {
				logger.Printf("Replicated ConfigMap %s to namespace %s", updatedConfigMap.Name, ns)
			}
			return
		} else {
			logger.Printf("Error fetching ConfigMap %s in namespace %s", updatedConfigMap.Name, ns)
			return
		}
	}

	_, err = c.clientset.CoreV1().ConfigMaps(ns).Update(ctx, updatedConfigMap, metav1.UpdateOptions{})
	if err != nil {
		logger.Printf("Error replicating ConfigMap to namespace %s: %v", ns, err)
	} else {
		logger.Printf("Updated ConfigMap %s in namespace %s", updatedConfigMap.Name, ns)
	}
}

func (c *ConfigMapReplicatorController) updateConfigMapAcrossNamespaces(ctx context.Context, currentConfigMap *v1.ConfigMap, updatedConfigMap *v1.ConfigMap) {
	if c.replicateEnabled(updatedConfigMap) {
		if len(*c.AllowedNamespaces) > 0 {
			for _, ns := range *c.AllowedNamespaces {
				// Update ConfigMap
				c.updateConfigMap(ctx, updatedConfigMap, ns)
			}
		} else {
			namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				logger.Printf("Error listing namespaces: %v", err)
				return
			}

			for _, ns := range namespaces.Items {
				if updatedConfigMap.Namespace == ns.Name {
					logger.Printf("ConfigMap %s in the %s namespace is a source ConfigMap", updatedConfigMap.Name, updatedConfigMap.Namespace)
					continue
				} else if utils.ListContains(*c.ExcludedNamespaces, ns.Name) {
					logger.Printf("Namespace %s is an excluded Namespace. Not replicating ConfigMap %s to Namespace %s.", ns.Name, updatedConfigMap.Name, ns.Name)
					continue
				} else {
					// Update ConfigMap
					c.updateConfigMap(ctx, updatedConfigMap, ns.Name)
				}
			}
		}
	}
}

func (c *ConfigMapReplicatorController) deleteConfigMapAcrossNamespaces(ctx context.Context, configMap *v1.ConfigMap) {
	if c.replicateEnabled(configMap) {
		namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			logger.Printf("Error listing namespaces: %v", err)
			return
		}

		for _, ns := range namespaces.Items {
			if configMap.Namespace == ns.Name {
				continue
			}

			if utils.ListContains(*c.ExcludedNamespaces, ns.Name) {
				logger.Printf("Namespace %s is an excluded Namespace. Not replicating ConfigMap %s to Namespace %s.", ns.Name, configMap.Name, ns.Name)
				continue
			} else {
				err = c.clientset.CoreV1().ConfigMaps(ns.Name).Delete(ctx, configMap.Name, metav1.DeleteOptions{})
				if err != nil {
					logger.Printf("Error deleting ConfigMap in namespace %s: %v", ns.Name, err)
				} else {
					logger.Printf("Deleted ConfigMap %s in namespace %s", configMap.Name, ns.Name)
				}
			}
		}
	}
}

func (c *ConfigMapReplicatorController) Run(ctx context.Context) error {
	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo metav1.ListOptions) (runtime.Object, error) {
				return c.clientset.CoreV1().ConfigMaps("").List(ctx, lo)
			},
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return c.clientset.CoreV1().ConfigMaps("").Watch(ctx, lo)
			},
		},
		&v1.ConfigMap{},
		*c.ReconciliationInterval,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				// Replicate the ConfigMap to all namespaces
				configMap := obj.(*v1.ConfigMap)
				c.addConfigMapAcrossNamespaces(ctx, configMap)
			},
			UpdateFunc: func(currentObj, newObj interface{}) {
				// Handle ConfigMap updates
				currentConfigMap := currentObj.(*v1.ConfigMap)
				updatedConfigMap := currentObj.(*v1.ConfigMap)
				c.updateConfigMapAcrossNamespaces(ctx, currentConfigMap, updatedConfigMap)
			},
			DeleteFunc: func(obj interface{}) {
				// Handle ConfigMap deletions
				configMap := obj.(*v1.ConfigMap)
				c.deleteConfigMapAcrossNamespaces(ctx, configMap)
			},
		},
	)

	controller.Run(wait.NeverStop)

	return nil
}
