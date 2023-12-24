package main

import (
	"github.com/dm0275/configmap-replicator/cmd/controller"
	_ "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	_ "k8s.io/client-go/tools/clientcmd"
	"log"
)

var logger = log.Default()

func main() {
	// Load Kubernetes configuration using the in cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatalf("Error loading in-cluster config: %v\n", err)
	}

	controller.Run(config)
}
