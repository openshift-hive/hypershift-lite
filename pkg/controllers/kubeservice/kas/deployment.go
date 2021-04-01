package kas

import (
	"fmt"
	"path"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/etcd"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/pki"
)

const (
	// containers in deployment
	initBootstrapContainer           = "init-bootstrap"  // init container
	applyBootstrapManifestsContainer = "apply-bootstrap" // side car
	kubeAPIServerContainer           = "kube-apiserver"  // main container

	// volumes
	bootstrapManifestsVolume  = "bootstrap-manifests"  // emptyDir volume where bootstrap config is placed
	localhostKubeconfigVolume = "localhost-kubeconfig" // secret containing localhost kubeconfig
	workLogsVolume            = "logs"                 // emptyDir volume where logs are written
	kasConfigVolume           = "kas-config"           // configMap containing kube apiserver config file
	auditConfigVolume         = "audit-config"
	rootCAVolume              = "root-ca"
	serverCertVolume          = "server-crt"
	aggregatorCertVolume      = "aggregator-crt"
	serviceAccountKeyVolume   = "svcacct-key"
	etcdClientCertVolume      = "etcd-client-crt"
	oauthMetadataVolume       = "oauth-metadata"

	// volume mounts in init bootstrap container
	initWorkMountPath = "/work" // manifests are saved here to be later applied by apply-bootstrap

	// volume mounts in apply bootstrap container
	applyWorkMountPath       = "/work"
	applyKubeconfigMountPath = "/var/secrets/localhost-kubeconfig"

	// volume mounts in kube apiserver
	kasWorkLogsMountPath          = "/var/log/kube-apiserver"
	kasConfigMountPath            = "/etc/kubernetes/config"
	kasAuditConfigMountPath       = "/etc/kubernetes/audit"
	kasRootCAMountPath            = "/etc/kubernetes/certs/root-ca"
	kasServerCertMountPath        = "/etc/kubernetes/certs/server"
	kasAggregatorCertMountPath    = "/etc/kubernetes/certs/aggregator"
	kasEtcdClientCertMountPath    = "/etc/kubernetes/certs/etcd"
	kasServiceAccountKeyMountPath = "/etc/kubernetes/secrets/svcacct-key"
	kasOauthMetadataMountPath     = "/etc/kubernetes/oauth"
)

var kasLabels = map[string]string{
	"app": "kube-apiserver",
}

func ReconcileKubeAPIServerDeployment(
	deployment *appsv1.Deployment,
	configOperatorImage string,
	cliImage string,
	hyperKubeImage string,
	internalAPIServerPort int,
	replicaCount int,
) error {
	maxSurge := intstr.FromInt(3)
	maxUnavailable := intstr.FromInt(1)
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: pointer.Int32Ptr(int32(replicaCount)),
		Selector: &metav1.LabelSelector{
			MatchLabels: kasLabels,
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
				Labels: kasLabels,
			},
			Spec: corev1.PodSpec{
				AutomountServiceAccountToken: pointer.BoolPtr(false),
				InitContainers: []corev1.Container{
					{
						Name:  initBootstrapContainer,
						Image: configOperatorImage,
						Command: []string{
							"/bin/bash",
						},
						Args: []string{
							"-c",
							invokeMCORenderScript(initWorkMountPath),
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      bootstrapManifestsVolume,
								MountPath: initWorkMountPath,
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:  applyBootstrapManifestsContainer,
						Image: cliImage,
						Command: []string{
							"/bin/bash",
						},
						Args: []string{
							"-c",
							applyBootstrapManifestsScript(applyWorkMountPath),
						},
						Env: []corev1.EnvVar{
							{
								Name:  "KUBECONFIG",
								Value: path.Join(applyKubeconfigMountPath, KubeconfigKey),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      bootstrapManifestsVolume,
								MountPath: applyWorkMountPath,
							},
							{
								Name:      localhostKubeconfigVolume,
								MountPath: applyKubeconfigMountPath,
							},
						},
					},
					{
						Name:  kubeAPIServerContainer,
						Image: hyperKubeImage,
						Command: []string{
							"hyperkube",
						},
						Args: []string{
							"kube-apiserver",
							fmt.Sprintf("--openshift-config=%s", path.Join(kasConfigMountPath, KubeAPIServerConfigKey)),
							"-v5",
						},
						WorkingDir: kasWorkLogsMountPath,
						LivenessProbe: &corev1.Probe{
							InitialDelaySeconds: 45,
							TimeoutSeconds:      10,
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/livez",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt(internalAPIServerPort),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt(internalAPIServerPort),
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      workLogsVolume,
								MountPath: kasWorkLogsMountPath,
							},
							{
								Name:      kasConfigVolume,
								MountPath: kasConfigMountPath,
							},
							{
								Name:      auditConfigVolume,
								MountPath: kasAuditConfigMountPath,
							},
							{
								Name:      rootCAVolume,
								MountPath: kasRootCAMountPath,
							},
							{
								Name:      serverCertVolume,
								MountPath: kasServerCertMountPath,
							},
							{
								Name:      aggregatorCertVolume,
								MountPath: kasAggregatorCertMountPath,
							},
							{
								Name:      etcdClientCertVolume,
								MountPath: kasEtcdClientCertMountPath,
							},
							{
								Name:      serviceAccountKeyVolume,
								MountPath: kasServiceAccountKeyMountPath,
							},
							{
								Name:      oauthMetadataVolume,
								MountPath: kasOauthMetadataMountPath,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: bootstrapManifestsVolume,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: localhostKubeconfigVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: LocalhostKubeconfigSecret(deployment.Namespace).Name,
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
						Name: kasConfigVolume,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: Config(deployment.Namespace).Name,
								},
							},
						},
					},
					{
						Name: auditConfigVolume,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: AuditConfig(deployment.Namespace).Name,
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
						Name: serverCertVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: ServerCertSecret(deployment.Namespace).Name,
							},
						},
					},
					{
						Name: aggregatorCertVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: AggregatorCertSecret(deployment.Namespace).Name,
							},
						},
					},
					{
						Name: serviceAccountKeyVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: ServiceAccountSigningKeySecret(deployment.Namespace).Name,
							},
						},
					},
					{
						Name: etcdClientCertVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: etcd.ClientSecret(deployment.Namespace).Name,
							},
						},
					},
					{
						Name: oauthMetadataVolume,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: OAuthMetadata(deployment.Namespace).Name,
								},
							},
						},
					},
				},
			},
		},
	}
	return nil
}

func invokeMCORenderScript(workDir string) string {
	var script = `#!/bin/sh
cd /tmp
mkdir input output
/usr/bin/cluster-config-operator render \
   --config-output-file config \
   --asset-input-dir /tmp/input \
   --asset-output-dir /tmp/output
cp /tmp/output/manifests/* %[1]s
`
	return fmt.Sprintf(script, workDir)
}

func applyBootstrapManifestsScript(workDir string) string {
	var script = `#!/bin/sh
while true; do
  if oc apply -f %[1]s; then
    echo "Bootstrap manifests applied successfully."
    break
  fi
  sleep 1
done
while true; do
  sleep 1000
done
`
	return fmt.Sprintf(script, workDir)
}
