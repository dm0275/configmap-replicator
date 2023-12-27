package controller

import (
	"context"
	"fmt"
	"github.com/dm0275/configmap-replicator/utils"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
	"time"
)

var (
	annotationKey         = "configmap-replicator"
	replicatedFromKey     = "replicated-from"
	replicationAllowedKey = "replication-allowed"
	allowedNamespacesKey  = "allowed-namespaces"
	excludedNamespacesKey = "excluded-namespaces"
)

// ConfigMapReplicatorController is responsible for replicating ConfigMaps.
type ConfigMapReplicatorController struct {
	clientset              *kubernetes.Clientset
	ReconciliationInterval *time.Duration
}

// NewConfigMapReplicatorController creates a new instance of the ConfigMapReplicatorController.
func NewConfigMapReplicatorController(config *rest.Config, reconciliationInterval string) *ConfigMapReplicatorController {
	interval, err := time.ParseDuration(reconciliationInterval)
	if err != nil {
		klog.Fatalf("Invalid reconciliation interval %s: %v\n", reconciliationInterval, err)
	}

	// Create a Kubernetes clientset.
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Error creating Kubernetes clientset: %v\n", err)
	}

	controller := &ConfigMapReplicatorController{
		clientset:              clientset,
		ReconciliationInterval: &interval,
	}

	return controller
}

func (c *ConfigMapReplicatorController) validateConfiguration(configMap *v1.ConfigMap) error {
	allowedNamespaces := c.getAllowedNamespaces(configMap)
	excludedNamespaces := c.getExcludedNamespaces(configMap)

	if utils.SlicesOverlap(allowedNamespaces, excludedNamespaces) {
		return fmt.Errorf("ERROR: Unable to replicate ConfigMap %s, cannot have overlaps between allowedNamespaces and excludedNamespaces", configMap.Name)
	}

	return nil
}

// Replicate the given ConfigMap to all namespaces
func (c *ConfigMapReplicatorController) addConfigMapAcrossNamespaces(ctx context.Context, configMap *v1.ConfigMap) {
	// Validate configmap configuration
	err := c.validateConfiguration(configMap)
	if err != nil {
		klog.Errorf(err.Error())
		return
	}

	if c.replicateEnabled(configMap) {
		allowedNamespaces := c.getAllowedNamespaces(configMap)
		if len(allowedNamespaces) > 0 {
			for _, ns := range allowedNamespaces {
				// Create a new ConfigMap
				go c.createConfigMap(ctx, configMap, ns)
			}
		} else {
			namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				klog.Errorf("Error listing namespaces: %v", err)
				return
			}

			for _, ns := range namespaces.Items {
				excludedNamespaces := c.getExcludedNamespaces(configMap)
				if configMap.Namespace == ns.Name {
					klog.Infof("ConfigMap %s in the %s namespace is a source ConfigMap", configMap.Name, configMap.Namespace)
					continue
				} else if utils.ListContains(excludedNamespaces, ns.Name) {
					klog.Infof("Namespace %s is an excluded Namespace. Not replicating ConfigMap %s to Namespace %s.", ns.Name, configMap.Name, ns.Name)
					continue
				} else {
					// Create a new ConfigMap in each namespace
					go c.createConfigMap(ctx, configMap, ns.Name)
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
				fmt.Sprintf("%s/%s", annotationKey, replicatedFromKey): configMap.Namespace + "_" + configMap.Name,
			},
		},
		Data: configMap.Data,
	}

	_, err := c.clientset.CoreV1().ConfigMaps(ns).Create(ctx, newConfigMap, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("Error replicating ConfigMap to namespace %s: %v", ns, err)
	} else {
		klog.Infof("Replicated ConfigMap %s to namespace %s", configMap.Name, ns)
	}
}

func (c *ConfigMapReplicatorController) updateConfigMap(ctx context.Context, configMap *v1.ConfigMap, ns string) {
	updatedConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMap.Name,
			Namespace: ns,
			Annotations: map[string]string{
				fmt.Sprintf("%s/%s", annotationKey, replicatedFromKey): configMap.Namespace + "_" + configMap.Name,
			},
		},
		Data: configMap.Data,
	}

	_, err := c.clientset.CoreV1().ConfigMaps(ns).Get(ctx, updatedConfigMap.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err = c.clientset.CoreV1().ConfigMaps(ns).Create(ctx, updatedConfigMap, metav1.CreateOptions{})
			if err != nil {
				klog.Errorf("Error replicating ConfigMap to namespace %s: %v", ns, err)
			} else {
				klog.Infof("Replicated ConfigMap %s to namespace %s", updatedConfigMap.Name, ns)
			}
			return
		} else {
			klog.Errorf("Error fetching ConfigMap %s in namespace %s", updatedConfigMap.Name, ns)
			return
		}
	}

	_, err = c.clientset.CoreV1().ConfigMaps(ns).Update(ctx, updatedConfigMap, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Error replicating ConfigMap to namespace %s: %v", ns, err)
	} else {
		klog.Infof("Updated ConfigMap %s in namespace %s", updatedConfigMap.Name, ns)
	}
}

func (c *ConfigMapReplicatorController) updateConfigMapAcrossNamespaces(ctx context.Context, currentConfigMap *v1.ConfigMap, updatedConfigMap *v1.ConfigMap) {
	// Validate configmap configuration
	err := c.validateConfiguration(updatedConfigMap)
	if err != nil {
		klog.Errorf(err.Error())
		return
	}

	if c.replicateEnabled(updatedConfigMap) {
		allowedNamespaces := c.getAllowedNamespaces(updatedConfigMap)
		if len(allowedNamespaces) > 0 {
			for _, ns := range allowedNamespaces {
				// Update ConfigMap
				go c.updateConfigMap(ctx, updatedConfigMap, ns)
			}
		} else {
			namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				klog.Errorf("Error listing namespaces: %v", err)
				return
			}

			for _, ns := range namespaces.Items {
				excludedNamespaces := c.getExcludedNamespaces(updatedConfigMap)
				if updatedConfigMap.Namespace == ns.Name {
					klog.Infof("ConfigMap %s in the %s namespace is a source ConfigMap", updatedConfigMap.Name, updatedConfigMap.Namespace)
					continue
				} else if utils.ListContains(excludedNamespaces, ns.Name) {
					klog.Infof("Namespace %s is an excluded Namespace. Not replicating ConfigMap %s to Namespace %s.", ns.Name, updatedConfigMap.Name, ns.Name)
					continue
				} else {
					// Update ConfigMap
					go c.updateConfigMap(ctx, updatedConfigMap, ns.Name)
				}
			}
		}
	}
}

func (c *ConfigMapReplicatorController) deleteConfigMapAcrossNamespaces(ctx context.Context, configMap *v1.ConfigMap) {
	// Validate configmap configuration
	err := c.validateConfiguration(configMap)
	if err != nil {
		klog.Errorf(err.Error())
		return
	}

	if c.replicateEnabled(configMap) {
		namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Error listing namespaces: %v", err)
			return
		}

		for _, ns := range namespaces.Items {
			if configMap.Namespace == ns.Name {
				continue
			}

			excludedNamespaces := c.getExcludedNamespaces(configMap)
			if utils.ListContains(excludedNamespaces, ns.Name) {
				klog.Infof("Namespace %s is an excluded Namespace. Not replicating ConfigMap %s to Namespace %s.", ns.Name, configMap.Name, ns.Name)
				continue
			} else {
				err = c.clientset.CoreV1().ConfigMaps(ns.Name).Delete(ctx, configMap.Name, metav1.DeleteOptions{})
				if err != nil {
					klog.Errorf("Error deleting ConfigMap in namespace %s: %v", ns.Name, err)
				} else {
					klog.Infof("Deleted ConfigMap %s in namespace %s", configMap.Name, ns.Name)
				}
			}
		}
	}
}

// Run starts the controller and watches for ConfigMap changes.
func (c *ConfigMapReplicatorController) Run(ctx context.Context) error {
	// The informer is used to watch and react to changes in resources, in this case ConfigMaps.
	_, controller := cache.NewInformer(
		// The first arg is a `cache.ListWatch` object. This object specifies how to list and watch for changes in the ConfigMaps.
		&cache.ListWatch{
			// The `ListFunc` is responsible for listing the ConfigMaps
			ListFunc: func(lo metav1.ListOptions) (runtime.Object, error) {
				return c.clientset.CoreV1().ConfigMaps("").List(ctx, lo)
			},
			// The `WatchFunc` is responsible for setting up a watch on the ConfigMaps. It returns a watch.Interface that will notify the controller of any changes to the watched resources.
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return c.clientset.CoreV1().ConfigMaps("").Watch(ctx, lo)
			},
		},
		// The second arg is the type of the resource we are watching. In this case, a ConfigMap.
		&v1.ConfigMap{},
		// The third arg is the resync period(time.Duration), this specifies how often the informer should perform a full re-list of the resources, even if no changes have occurred. This helps ensure that your controller has up-to-date information.
		*c.ReconciliationInterval,
		// The fourth arg is a set of event handler functions. These functions define what happens when resources are added, updated, or deleted.
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

	// Start the controller and make it run indefinitely to continuously monitor resources as changes occur in the cluster.
	controller.Run(wait.NeverStop)

	return nil
}

func (c *ConfigMapReplicatorController) replicateEnabled(configMap *v1.ConfigMap) bool {
	replicationAllowed, ok := configMap.Annotations[fmt.Sprintf("%s/%s", annotationKey, replicationAllowedKey)]
	if !ok {
		return false
	}

	replicationAllowedBool, err := strconv.ParseBool(replicationAllowed)
	if err != nil {
		return false
	}

	return replicationAllowedBool
}

func (c *ConfigMapReplicatorController) getAllowedNamespaces(configMap *v1.ConfigMap) []string {
	allowedNamespaces, ok := configMap.Annotations[fmt.Sprintf("%s/%s", annotationKey, allowedNamespacesKey)]
	if !ok {
		return []string{}
	}

	return strings.Split(allowedNamespaces, ",")
}

func (c *ConfigMapReplicatorController) getExcludedNamespaces(configMap *v1.ConfigMap) []string {
	excludedNamespaces, ok := configMap.Annotations[fmt.Sprintf("%s/%s", annotationKey, excludedNamespacesKey)]
	if !ok {
		return []string{}
	}

	return strings.Split(excludedNamespaces, ",")
}
