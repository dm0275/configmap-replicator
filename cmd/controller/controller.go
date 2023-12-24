package controller

import (
	"context"
	configMapcontroller "github.com/dm0275/configmap-replicator/pkg/controller"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"log"
)

var logger = log.Default()

type ControllerConfig struct {
	ReconciliationInterval string
	ExcludedNamespaces     []string
	AllowedNamespaces      []string
}

func Run(config *rest.Config) {
	controllerConfig := &ControllerConfig{}
	cmd := &cobra.Command{
		Use: "configmap-replicator",
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize Controller
			controller := configMapcontroller.NewConfigMapReplicatorController(config, controllerConfig.ReconciliationInterval, controllerConfig.ExcludedNamespaces, controllerConfig.AllowedNamespaces)

			// Initialize context
			ctx := context.Background()

			// Start the controller.
			if err := controller.Run(ctx); err != nil {
				logger.Fatalf("Error running controller: %v\n", err)
			}
		},
	}

	configureFlags(cmd, controllerConfig)

	cobra.CheckErr(cmd.Execute())
}

func configureFlags(cmd *cobra.Command, config *ControllerConfig) {
	cmd.Flags().StringVarP(&config.ReconciliationInterval, "reconciliation-interval", "", "1m", "configures the reconciliation interval of the controller")
	cmd.Flags().StringSliceVar(&config.ExcludedNamespaces, "excluded-namespaces", []string{"kube-system"}, "namespaces excluded from replication")
	cmd.Flags().StringSliceVar(&config.AllowedNamespaces, "allowed-namespaces", []string{}, "configures exclusive namespaces allowed for replication")
}
