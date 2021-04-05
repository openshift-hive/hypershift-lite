package main

import (
	"fmt"
	"os"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/spf13/cobra"

	hyperliteapi "github.com/openshift-hive/hypershiftlite/pkg/api"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	cmd := HypershiftLiteCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func HypershiftLiteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "hypershift-lite",
		Run: runHypershiftLite,
	}
	return cmd
}

func runHypershiftLite(cmd *cobra.Command, args []string) {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: hyperliteapi.Scheme,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&kubeservice.KubernetesServiceReconciler{
		Client: mgr.GetClient(),
		Config: mgr.GetConfig(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "hosted-control-plane")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

type uncachedClientBuilder struct{}

func (n *uncachedClientBuilder) WithUncached(_ ...client.Object) cluster.ClientBuilder {
	return n
}

func (n *uncachedClientBuilder) Build(_ cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
	c, err := client.New(config, options)
	if err != nil {
		return nil, err
	}
	return c, nil
}
