package main

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"os"
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
	if err := controller.Run(); err != nil {
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
		"source-namespace", // Change to your source namespace.
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				// Replicate the ConfigMap to target namespaces.
				// Add your replication logic here.
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				// Handle updates to ConfigMaps.
			},
			DeleteFunc: func(obj interface{}) {
				// Handle ConfigMap deletions if necessary.
			},
		},
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
