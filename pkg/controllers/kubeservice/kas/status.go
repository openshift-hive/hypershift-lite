package kas

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hyperlitev1 "github.com/openshift-hive/hypershiftlite/pkg/api/v1alpha1"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/ks"
)

func ReconcileKubeAPIServerDeploymentStatus(ctx context.Context, c client.Client, kubeSvc *hyperlitev1.KubernetesService, deployment *appsv1.Deployment) error {
	log := ctrl.LoggerFrom(ctx)
	if deployment == nil {
		log.Info("Kube APIServer deployment doesn't exist yet")
		return nil
	}
	availableCondition := ks.DeploymentConditionByType(deployment, appsv1.DeploymentAvailable)
	if availableCondition != nil && availableCondition.Status == corev1.ConditionTrue &&
		deployment.Status.AvailableReplicas > 0 {
		ks.SetConditionByType(&kubeSvc.Status.Conditions, hyperlitev1.KubeAPIServerAvailable, corev1.ConditionTrue, "KASRunning", "Kube APIServer is running and available")
	} else {
		ks.SetConditionByType(&kubeSvc.Status.Conditions, hyperlitev1.KubeAPIServerAvailable, corev1.ConditionFalse, "KASScalingUp", "Kube APIServer is not yet ready")
	}
	if err := c.Status().Update(ctx, kubeSvc); err != nil {
		return err
	}
	return nil
}
