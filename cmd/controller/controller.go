package controller

import (
	"context"
	configMapcontroller "github.com/dm0275/configmap-replicator/pkg/controller"
	"github.com/dm0275/configmap-replicator/utils"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type ControllerConfig struct {
	ReconciliationInterval string
}

func Run(config *rest.Config) {
	controllerConfig := &ControllerConfig{}
	cmd := &cobra.Command{
		Use: "configmap-replicator",
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize Controller
			controller := configMapcontroller.NewConfigMapReplicatorController(config, controllerConfig.ReconciliationInterval)

			// Initialize context
			ctx := context.Background()

			// Start the controller.
			if err := controller.Run(ctx); err != nil {
				klog.Fatalf("Error running controller: %v\n", err)
			}
		},
	}

	configureFlags(cmd, controllerConfig)

	cobra.CheckErr(cmd.Execute())
}

func configureFlags(cmd *cobra.Command, config *ControllerConfig) {
	reconciliationInterval := utils.GetEnv("REPLICATOR_INTERVAL", "1m")

	cmd.Flags().StringVarP(&config.ReconciliationInterval, "reconciliation-interval", "", reconciliationInterval, "configures the reconciliation interval of the controller")
}
