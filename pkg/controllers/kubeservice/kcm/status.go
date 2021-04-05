package kcm

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hyperlitev1 "github.com/openshift-hive/hypershiftlite/pkg/api/v1alpha1"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/ks"
)

func ReconcileKubeControllerManagerDeploymentStatus(ctx context.Context, c client.Client, kubeSvc *hyperlitev1.KubernetesService, deployment *appsv1.Deployment) error {
	log := ctrl.LoggerFrom(ctx)
	if deployment == nil {
		log.Info("Kube APIServer deployment doesn't exist yet")
		return nil
	}
	availableCondition := ks.DeploymentConditionByType(deployment, appsv1.DeploymentAvailable)
	if availableCondition != nil && availableCondition.Status == corev1.ConditionTrue &&
		deployment.Status.AvailableReplicas > 0 {
		ks.SetConditionByType(&kubeSvc.Status.Conditions, hyperlitev1.KubeControllerManagerAvailable, corev1.ConditionTrue, "KCMRunning", "Kube controller manager is running and available")
	} else {
		ks.SetConditionByType(&kubeSvc.Status.Conditions, hyperlitev1.KubeControllerManagerAvailable, corev1.ConditionFalse, "KCMScalingUp", "Kube controller manager is not yet ready")
	}
	if err := c.Status().Update(ctx, kubeSvc); err != nil {
		return err
	}
	return nil
}
