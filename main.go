package main

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
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

func main() {
	// Load Kubernetes configuration from the default location or from a kubeconfig file.
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatalf("Error loading in-cluster config: %v\n", err)
	}

	// Create a Kubernetes clientset.
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatalf("Error creating Kubernetes clientset: %v\n", err)
	}

	// Create a ConfigMap controller.
	reconciliationInterval := "5m"
	controller := NewConfigMapReplicatorController(clientset, reconciliationInterval)

	// Start the controller.
	if err = controller.Run(); err != nil {
		logger.Fatalf("Error running controller: %v\n", err)
	}
}

// ConfigMapReplicatorController is responsible for replicating ConfigMaps.
type ConfigMapReplicatorController struct {
	clientset              *kubernetes.Clientset
	reconciliationInterval time.Duration
}

// NewConfigMapReplicatorController creates a new instance of the ConfigMapReplicatorController.
func NewConfigMapReplicatorController(clientset *kubernetes.Clientset, reconciliationInterval string) *ConfigMapReplicatorController {
	interval, err := time.ParseDuration(reconciliationInterval)
	if err != nil {
		panic(err)
	}
	return &ConfigMapReplicatorController{
		clientset:              clientset,
		reconciliationInterval: interval,
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
func (c *ConfigMapReplicatorController) addConfigMapAcrossNamespaces(configMap *v1.ConfigMap) {
	if c.replicateEnabled(configMap) {
		namespaces, err := c.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logger.Printf("Error listing namespaces: %v", err)
			return
		}

		for _, ns := range namespaces.Items {
			if configMap.Namespace == ns.Name {
				continue
			} else {
				// Create a new ConfigMap in each namespace
				newConfigMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configMap.Name,
						Namespace: ns.Name,
					},
					Data: configMap.Data,
				}

				_, err = c.clientset.CoreV1().ConfigMaps(ns.Name).Create(context.TODO(), newConfigMap, metav1.CreateOptions{})
				if err != nil {
					logger.Printf("Error replicating ConfigMap to namespace %s: %v", ns.Name, err)
				} else {
					logger.Printf("Replicated ConfigMap %s to namespace %s", configMap.Name, ns.Name)
				}
			}
		}
	} else {
		logger.Printf("Replication is not allowed for ConfigMap %s", configMap.Name)
	}
}

func (c *ConfigMapReplicatorController) updateConfigMapAcrossNamespaces(currentConfigMap *v1.ConfigMap, updatedConfigMap *v1.ConfigMap) {
	if c.replicateEnabled(updatedConfigMap) {
		namespaces, err := c.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logger.Printf("Error listing namespaces: %v", err)
			return
		}

		for _, ns := range namespaces.Items {
			if updatedConfigMap.Namespace == ns.Name {
				continue
			} else {
				configMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      updatedConfigMap.Name,
						Namespace: ns.Name,
					},
					Data: updatedConfigMap.Data,
				}

				_, err := c.clientset.CoreV1().ConfigMaps(ns.Name).Update(context.TODO(), configMap, metav1.UpdateOptions{})
				if err != nil {
					logger.Printf("Error replicating ConfigMap to namespace %s: %v", ns.Name, err)
				} else {
					logger.Printf("Updated ConfigMap %s in namespace %s", configMap.Name, ns.Name)
				}
			}
		}
	} else {
		logger.Printf("Replication is not allowed for ConfigMap %s", updatedConfigMap.Name)
	}
}

func (c *ConfigMapReplicatorController) Run() error {
	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo metav1.ListOptions) (runtime.Object, error) {
				return c.clientset.CoreV1().ConfigMaps("").List(context.TODO(), lo)
			},
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return c.clientset.CoreV1().ConfigMaps("").Watch(context.TODO(), lo)
			},
		},
		&v1.ConfigMap{},
		c.reconciliationInterval,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				// Replicate the ConfigMap to all namespaces
				configMap := obj.(*v1.ConfigMap)
				c.addConfigMapAcrossNamespaces(configMap)
			},
			UpdateFunc: func(currentObj, newObj interface{}) {
				// Handle ConfigMap updates
				currentConfigMap := currentObj.(*v1.ConfigMap)
				updatedConfigMap := currentObj.(*v1.ConfigMap)
				c.updateConfigMapAcrossNamespaces(currentConfigMap, updatedConfigMap)
			},
			DeleteFunc: func(obj interface{}) {
				// Handle ConfigMap deletions
				// ...
				configMap := obj.(*v1.ConfigMap)
				fmt.Printf("Configmap %s deleted", configMap.Name)
				logger.Printf("Configmap %s deleted", configMap.Name)
			},
		},
	)

	controller.Run(wait.NeverStop)

	return nil
}
