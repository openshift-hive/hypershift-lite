package kas

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ServerCertSecret(controlPlaneNamespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kas-server-crt",
			Namespace: controlPlaneNamespace,
		},
	}
}

func AggregatorCertSecret(controlPlaneNamespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kas-aggregator-crt",
			Namespace: controlPlaneNamespace,
		},
	}
}

func ServiceAccountSigningKeySecret(controlPlaneNamespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kas-sa-key",
			Namespace: controlPlaneNamespace,
		},
	}
}

func LocalhostKubeconfigSecret(controlPlaneNamespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "localhost-kubeconfig",
			Namespace: controlPlaneNamespace,
		},
	}
}

func ServiceKubeconfigSecret(controlPlaneNamespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig",
			Namespace: controlPlaneNamespace,
		},
	}
}

func Deployment(controlPlaneNamespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver",
			Namespace: controlPlaneNamespace,
		},
	}
}

func AuditConfig(controlPlaneNamespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kas-audit-config",
			Namespace: controlPlaneNamespace,
		},
	}
}

func Config(controlPlaneNamespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kas-config",
			Namespace: controlPlaneNamespace,
		},
	}
}

func Service(controlPlaneNamespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver",
			Namespace: controlPlaneNamespace,
		},
	}
}

func OAuthMetadata(controlPlaneNamespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oauth-metadata",
			Namespace: controlPlaneNamespace,
		},
	}
}
