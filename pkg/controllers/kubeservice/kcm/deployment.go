package kcm

import (
	"fmt"
	"path"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/kas"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/pki"
)

const (
	// containers in deployment
	kubeControllerManagerContainer = "kube-controller-manager" // main container

	// volumes
	kcmConfigVolume     = "kcm-config"
	rootCAVolume        = "root-ca"
	workLogsVolume      = "logs"
	kubeconfigVolume    = "kubeconfig"
	certDirVolume       = "certs"
	clusterSignerVolume = "cluster-signer"
	serviceSignerVolume = "service-signer"

	// volume mounts
	kcmConfigMountPath        = "/etc/kubernetes/config"
	kcmRootCAMountPath        = "/etc/kubernetes/certs/root-ca"
	kcmWorkLogsMountPath      = "/var/log/kube-controller-manager"
	kcmKubeconfigMountPath    = "/etc/kubernetes/secrets/svc-kubeconfig"
	kcmCertDirMountPath       = "/var/run/kubernetes"
	kcmClusterSignerMountPath = "/etc/kubernetes/certs/cluster-signer"
	kcmServiceSignerMountPath = "/etc/kubernetes/certs/service-signer"
)

var kcmLabels = map[string]string{
	"app": "kube-controller-manager",
}

func ReconcileDeployment(
	deployment *appsv1.Deployment,
	podCIDR string,
	serviceCIDR string,
	hyperKubeImage string,
	replicaCount int,
) error {
	maxSurge := intstr.FromInt(3)
	maxUnavailable := intstr.FromInt(1)
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: pointer.Int32Ptr(int32(replicaCount)),
		Selector: &metav1.LabelSelector{
			MatchLabels: kcmLabels,
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: kcmLabels,
			},
			Spec: corev1.PodSpec{
				AutomountServiceAccountToken: pointer.BoolPtr(false),
				Containers: []corev1.Container{
					{
						Name:  kubeControllerManagerContainer,
						Image: hyperKubeImage,
						Command: []string{
							"hyperkube",
							"kube-controller-manager",
						},
						Args: kcmArgs(podCIDR, serviceCIDR),
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      kcmConfigVolume,
								MountPath: kcmConfigMountPath,
							},
							{
								Name:      rootCAVolume,
								MountPath: kcmRootCAMountPath,
							},
							{
								Name:      workLogsVolume,
								MountPath: kcmWorkLogsMountPath,
							},
							{
								Name:      kubeconfigVolume,
								MountPath: kcmKubeconfigMountPath,
							},
							{
								Name:      clusterSignerVolume,
								MountPath: kcmClusterSignerMountPath,
							},
							{
								Name:      certDirVolume,
								MountPath: kcmCertDirMountPath,
							},
							{
								Name:      serviceSignerVolume,
								MountPath: kcmServiceSignerMountPath,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: kcmConfigVolume,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: Config(deployment.Namespace).Name,
								},
							},
						},
					},
					{
						Name: rootCAVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: pki.RootCASecret(deployment.Namespace).Name,
							},
						},
					},
					{
						Name: workLogsVolume,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: kubeconfigVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: kas.ServiceKubeconfigSecret(deployment.Namespace).Name,
							},
						},
					},
					{
						Name: clusterSignerVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: ClusterSignerSecret(deployment.Namespace).Name,
							},
						},
					},
					{
						Name: certDirVolume,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: serviceSignerVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: kas.ServiceAccountSigningKeySecret(deployment.Namespace).Name,
							},
						},
					},
				},
			},
		},
	}
	return nil
}

func kcmArgs(serviceCIDR, podCIDR string) []string {
	kubeConfigPath := path.Join(kcmKubeconfigMountPath, kas.KubeconfigKey)
	args := []string{
		fmt.Sprintf("--openshift-config=%s", path.Join(kcmConfigMountPath, KubeControllerManagerConfigKey)),
		fmt.Sprintf("--kubeconfig=%s", kubeConfigPath),
		fmt.Sprintf("--authentication-kubeconfig=%s", kubeConfigPath),
		fmt.Sprintf("--authorization-kubeconfig=%s", kubeConfigPath),
		"--allocate-node-cidrs=true",
		fmt.Sprintf("--cert-dir=%s", kcmCertDirMountPath),
		fmt.Sprintf("--cluster-cidr=%s", podCIDR),
		fmt.Sprintf("--cluster-signing-cert-file=%s", path.Join(kcmClusterSignerMountPath, SignerSecretCertKey)),
		fmt.Sprintf("--cluster-signing-key-file=%s", path.Join(kcmClusterSignerMountPath, SignerSecretKeyKey)),
		"--configure-cloud-routes=false",
		"--controllers=*",
		"--controllers=-ttl",
		"--controllers=-bootstrapsigner",
		"--controllers=-tokencleaner",
		"--enable-dynamic-provisioning=true",
		"--kube-api-burst=300",
		"--kube-api-qps=150",
		"--leader-elect-resource-lock=configmaps",
		"--leader-elect=true",
		"--leader-elect-retry-period=3s",
		"--port=0",
		fmt.Sprintf("--root-ca-file=%s", path.Join(kcmRootCAMountPath, pki.CASignerCertMapKey)),
		"--secure-port=10257",
		fmt.Sprintf("--service-account-private-key-file=%s", path.Join(kcmServiceSignerMountPath, kas.ServiceSignerPrivateKey)),
		fmt.Sprintf("--service-cluster-ip-range=%s", serviceCIDR),
		"--use-service-account-credentials=true",
		"--experimental-cluster-signing-duration=26280h",
	}
	for _, f := range kas.DefaultFeatureGates {
		args = append(args, fmt.Sprintf("--feature-gates=%s", f))
	}
	return args
}
