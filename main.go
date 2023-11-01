package main

import (
	"context"
	configMapcontroller "github.com/dm0275/configmap-replicator-operator/pkg/controller"
	_ "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	_ "k8s.io/client-go/tools/clientcmd"
	"log"
)

var logger = log.Default()

func main() {
	// Load Kubernetes configuration from the default location or from a kubeconfig file.
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatalf("Error loading in-cluster config: %v\n", err)
	}

	// Create a ConfigMap controller.
	reconciliationInterval := "1m"
	excludedNamespaces := []string{"kube-system"}
	allowedNamespaces := []string{}

	// Initialize Controller
	controller := configMapcontroller.NewConfigMapReplicatorController(config, reconciliationInterval, excludedNamespaces, allowedNamespaces)

	// Initialize context
	ctx := context.Background()

	// Start the controller.
	if err = controller.Run(ctx); err != nil {
		logger.Fatalf("Error running controller: %v\n", err)
	}
}
