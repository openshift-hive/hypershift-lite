package ks

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hyperlitev1 "github.com/openshift-hive/hypershiftlite/pkg/api/v1alpha1"
)

func SetConditionByType(conditions *[]hyperlitev1.KubernetesServiceCondition, conditionType hyperlitev1.ConditionType, status corev1.ConditionStatus, reason, message string) {
	existingCondition := GetConditionByType(*conditions, conditionType)
	if existingCondition == nil {
		newCondition := hyperlitev1.KubernetesServiceCondition{
			Type:               conditionType,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: metav1.Now(),
		}
		*conditions = append(*conditions, newCondition)
	} else {
		if existingCondition.Status != status {
			existingCondition.LastTransitionTime = metav1.Now()
		}
		existingCondition.Status = status
		existingCondition.Reason = reason
		existingCondition.Message = message
	}
}

func GetConditionByType(conditions []hyperlitev1.KubernetesServiceCondition, conditionType hyperlitev1.ConditionType) *hyperlitev1.KubernetesServiceCondition {
	for k, v := range conditions {
		if v.Type == conditionType {
			return &conditions[k]
		}
	}
	return nil
}

func DeploymentConditionByType(deployment *appsv1.Deployment, conditionType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i, c := range deployment.Status.Conditions {
		if c.Type == conditionType {
			return &deployment.Status.Conditions[i]
		}
	}
	return nil
}
