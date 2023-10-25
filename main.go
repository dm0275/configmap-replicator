package main

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"log"
	"os"
	"time"
)

func main() {
	// Load Kubernetes configuration from the default location or from a kubeconfig file.
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading in-cluster config: %v\n", err)
		os.Exit(1)
	}

	// Create a Kubernetes clientset.
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes clientset: %v\n", err)
		os.Exit(1)
	}

	// Create a ConfigMap controller.
	controller := NewConfigMapReplicatorController(clientset)

	// Start the controller.
	if err := controller.RunV3(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running controller: %v\n", err)
		os.Exit(1)
	}
}

// ConfigMapReplicatorController is responsible for replicating ConfigMaps.
type ConfigMapReplicatorController struct {
	clientset *kubernetes.Clientset
}

// NewConfigMapReplicatorController creates a new instance of the ConfigMapReplicatorController.
func NewConfigMapReplicatorController(clientset *kubernetes.Clientset) *ConfigMapReplicatorController {
	return &ConfigMapReplicatorController{clientset: clientset}
}

// Run starts the controller and watches for ConfigMap changes.
func (c *ConfigMapReplicatorController) Run() error {
	// Set up a ConfigMap watcher.
	configMapListWatcher := cache.NewListWatchFromClient(
		c.clientset.CoreV1().RESTClient(),
		"configmaps",
		"default", // Change to your source namespace.
		fields.Everything(),
	)

	// Set up a shared informer and run it in the background.
	sharedInformer := cache.NewSharedInformer(configMapListWatcher, &v1.ConfigMap{}, 0)
	go sharedInformer.Run(context.Background().Done())

	// Wait for the informer to sync.
	if !cache.WaitForCacheSync(context.Background().Done(), sharedInformer.HasSynced) {
		return fmt.Errorf("failed to sync informer cache")
	}

	// Block the main goroutine to keep the controller running.
	select {}
}

func (c *ConfigMapReplicatorController) RunV2() error {
	// Create a watch on ConfigMaps
	informerFactory := informers.NewSharedInformerFactory(c.clientset, 0)
	informer := informerFactory.Core().V1().ConfigMaps().Informer()

	fmt.Println("HERE")

	// Set up event handlers
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// Replicate the ConfigMap to all namespaces
			configMap := obj.(*v1.ConfigMap)
			fmt.Printf("Configmap %s found", configMap.Name)
			c.replicateConfigMapAcrossNamespaces(configMap)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// Handle ConfigMap updates
			// ...
		},
		DeleteFunc: func(obj interface{}) {
			// Handle ConfigMap deletions
			// ...
		},
	})
	if err != nil {
		return err
	}

	// Start the informer
	stopCh := make(chan struct{})
	defer close(stopCh)
	go informer.Run(stopCh)

	// Keep the controller running
	select {}
}

// Replicate the given ConfigMap to all namespaces
func (c *ConfigMapReplicatorController) replicateConfigMapAcrossNamespaces(configMap *v1.ConfigMap) {
	namespaces, err := c.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing namespaces: %v", err)
		return
	}

	for _, ns := range namespaces.Items {
		// Create a new ConfigMap in each namespace
		newConfigMap := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMap.Name,
				Namespace: ns.Name,
			},
			Data: configMap.Data,
		}

		_, err := c.clientset.CoreV1().ConfigMaps(ns.Name).Create(context.TODO(), newConfigMap, metav1.CreateOptions{})
		if err != nil {
			fmt.Printf("Error replicating ConfigMap to namespace %s: %v", ns.Name, err)
		} else {
			fmt.Printf("Replicated ConfigMap %s to namespace %s", configMap.Name, ns.Name)
		}
	}
}

func (c *ConfigMapReplicatorController) RunV3() error {
	logger := log.Default()
	resyncPeriod, err := time.ParseDuration("1m")
	if err != nil {
		panic(err)
	}

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
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				// Replicate the ConfigMap to all namespaces
				configMap := obj.(*v1.ConfigMap)
				fmt.Printf("Configmap %s added", configMap.Name)
				logger.Printf("Configmap %s added", configMap.Name)
				c.replicateConfigMapAcrossNamespaces(configMap)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				// Handle ConfigMap updates
				// ...
				configMap := oldObj.(*v1.ConfigMap)
				fmt.Printf("Configmap %s updated", configMap.Name)
				logger.Printf("Configmap %s updated", configMap.Name)
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
