package main

import (
	"context"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	_ "k8s.io/client-go/tools/clientcmd"
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
	reconciliationInterval := "1m"
	excludedNamespaces := []string{"kube-system"}
	allowedNamespaces := []string{}
	controller := NewConfigMapReplicatorController(clientset, reconciliationInterval, excludedNamespaces, allowedNamespaces)

	// Validate Controller configuration
	controller.configureNamespaces()

	// Start the controller.
	if err = controller.Run(); err != nil {
		logger.Fatalf("Error running controller: %v\n", err)
	}
}

// ConfigMapReplicatorController is responsible for replicating ConfigMaps.
type ConfigMapReplicatorController struct {
	clientset              *kubernetes.Clientset
	reconciliationInterval *time.Duration
	excludedNamespaces     *[]string
	allowedNamespaces      *[]string
}

func (c *ConfigMapReplicatorController) configureNamespaces() {
	if slicesOverlap(*c.allowedNamespaces, *c.excludedNamespaces) {
		logger.Fatalf("ERROR: Cannot have overlaps between allowedNamespaces and excludedNamespaces")
	}
}

// NewConfigMapReplicatorController creates a new instance of the ConfigMapReplicatorController.
func NewConfigMapReplicatorController(clientset *kubernetes.Clientset, reconciliationInterval string, excludedNamespaces, allowedNamespaces []string) *ConfigMapReplicatorController {
	interval, err := time.ParseDuration(reconciliationInterval)
	if err != nil {
		panic(err)
	}
	return &ConfigMapReplicatorController{
		clientset:              clientset,
		reconciliationInterval: &interval,
		excludedNamespaces:     &excludedNamespaces,
		allowedNamespaces:      &allowedNamespaces,
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
		if len(*c.allowedNamespaces) > 0 {
			for _, ns := range *c.allowedNamespaces {
				// Create a new ConfigMap
				c.createConfigMap(configMap, ns)
			}
		} else {
			namespaces, err := c.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				logger.Printf("Error listing namespaces: %v", err)
				return
			}

			for _, ns := range namespaces.Items {
				if configMap.Namespace == ns.Name {
					logger.Printf("ConfigMap %s in the %s namespace is a source ConfigMap", configMap.Name, configMap.Namespace)
					continue
				} else if listContains(*c.excludedNamespaces, ns.Name) {
					logger.Printf("Namespace %s is an excluded Namespace. Not replicating ConfigMap %s to Namespace %s.", ns.Name, configMap.Name, ns.Name)
					continue
				} else {
					// Create a new ConfigMap in each namespace
					c.createConfigMap(configMap, ns.Name)
				}
			}
		}
	}
}

func (c *ConfigMapReplicatorController) createConfigMap(configMap *v1.ConfigMap, ns string) {
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

	_, err := c.clientset.CoreV1().ConfigMaps(ns).Create(context.TODO(), newConfigMap, metav1.CreateOptions{})
	if err != nil {
		logger.Printf("Error replicating ConfigMap to namespace %s: %v", ns, err)
	} else {
		logger.Printf("Replicated ConfigMap %s to namespace %s", configMap.Name, ns)
	}
}

func (c *ConfigMapReplicatorController) updateConfigMap(configMap *v1.ConfigMap, ns string) {
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

	_, err := c.clientset.CoreV1().ConfigMaps(ns).Get(context.TODO(), updatedConfigMap.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err = c.clientset.CoreV1().ConfigMaps(ns).Create(context.TODO(), updatedConfigMap, metav1.CreateOptions{})
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

	_, err = c.clientset.CoreV1().ConfigMaps(ns).Update(context.TODO(), updatedConfigMap, metav1.UpdateOptions{})
	if err != nil {
		logger.Printf("Error replicating ConfigMap to namespace %s: %v", ns, err)
	} else {
		logger.Printf("Updated ConfigMap %s in namespace %s", updatedConfigMap.Name, ns)
	}
}

func (c *ConfigMapReplicatorController) updateConfigMapAcrossNamespaces(currentConfigMap *v1.ConfigMap, updatedConfigMap *v1.ConfigMap) {
	if c.replicateEnabled(updatedConfigMap) {
		if len(*c.allowedNamespaces) > 0 {
			for _, ns := range *c.allowedNamespaces {
				c.updateConfigMap(updatedConfigMap, ns)
			}
		} else {
			namespaces, err := c.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				logger.Printf("Error listing namespaces: %v", err)
				return
			}

			for _, ns := range namespaces.Items {
				if updatedConfigMap.Namespace == ns.Name {
					logger.Printf("ConfigMap %s in the %s namespace is a source ConfigMap", updatedConfigMap.Name, updatedConfigMap.Namespace)
					continue
				} else if listContains(*c.excludedNamespaces, ns.Name) {
					logger.Printf("Namespace %s is an excluded Namespace. Not replicating ConfigMap %s to Namespace %s.", ns.Name, updatedConfigMap.Name, ns.Name)
					continue
				} else {
					c.updateConfigMap(updatedConfigMap, ns.Name)
				}
			}
		}
	}
}

func (c *ConfigMapReplicatorController) deleteConfigMapAcrossNamespaces(configMap *v1.ConfigMap) {
	if c.replicateEnabled(configMap) {
		namespaces, err := c.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logger.Printf("Error listing namespaces: %v", err)
			return
		}

		for _, ns := range namespaces.Items {
			if configMap.Namespace == ns.Name {
				continue
			}

			if listContains(*c.excludedNamespaces, ns.Name) {
				logger.Printf("Namespace %s is an excluded Namespace. Not replicating ConfigMap %s to Namespace %s.", ns.Name, configMap.Name, ns.Name)
				continue
			} else {
				err = c.clientset.CoreV1().ConfigMaps(ns.Name).Delete(context.TODO(), configMap.Name, metav1.DeleteOptions{})
				if err != nil {
					logger.Printf("Error deleting ConfigMap in namespace %s: %v", ns.Name, err)
				} else {
					logger.Printf("Deleted ConfigMap %s in namespace %s", configMap.Name, ns.Name)
				}
			}
		}
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
		*c.reconciliationInterval,
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
				configMap := obj.(*v1.ConfigMap)
				c.deleteConfigMapAcrossNamespaces(configMap)
			},
		},
	)

	controller.Run(wait.NeverStop)

	return nil
}

func listContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Check if two slices have overlapping elements
func slicesOverlap(slice1, slice2 []string) bool {
	// Create a map to store elements from slice1
	seen := make(map[string]bool)

	// Populate the map with elements from slice1
	for _, elem := range slice1 {
		seen[elem] = true
	}

	// Check if any element from slice2 exists in the map
	for _, elem := range slice2 {
		if seen[elem] {
			return true
		}
	}

	return false
}
