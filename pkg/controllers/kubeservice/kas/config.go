package kas

import (
	"encoding/json"
	"fmt"
	"path"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	configv1 "github.com/openshift/api/config/v1"
	kcpv1 "github.com/openshift/api/kubecontrolplane/v1"

	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/etcd"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/pki"
)

const (
	KubeAPIServerConfigKey = "config.json"
	OauthMetadataConfigKey = "oauthMetadata.json"
	AuditLogFile           = "audit.log"
	DefaultEtcdPort        = 2379
)

const oauthMetadata = `{
"issuer": "https://oauth-openshift",
"authorization_endpoint": "https://oauth-openshift/oauth/authorize",
"token_endpoint": "https://oauth-openshift/oauth/token",
  "scopes_supported": [
    "user:check-access",
    "user:full",
    "user:info",
    "user:list-projects",
    "user:list-scoped-projects"
  ],
  "response_types_supported": [
    "code",
    "token"
  ],
  "grant_types_supported": [
    "authorization_code",
    "implicit"
  ],
  "code_challenge_methods_supported": [
    "plain",
    "S256"
  ]
}
`

var (
	DefaultFeatureGates = kcpv1.Arguments{
		"APIPriorityAndFairness=true",
		"RotateKubeletServerCertificate=true",
		"SupportPodPidsLimit=true",
		"NodeDisruptionExclusion=true",
		"ServiceNodeExclusion=true",
		"DownwardAPIHugePages=true",
		"LegacyNodeRoleBehavior=false",
	}
)

func ReconcileConfig(config *corev1.ConfigMap, serviceCIDR string, internalAPIServerPort int) error {
	if config.Data == nil {
		config.Data = map[string]string{}
	}
	serializedConfig, err := generateConfig(&ConfigParams{
		InternalAPIServerPort: internalAPIServerPort,
		Namespace:             config.Namespace,
		ServiceCIDR:           serviceCIDR,
	})
	if err != nil {
		return fmt.Errorf("failed to create apiserver config: %w", err)
	}
	config.Data[KubeAPIServerConfigKey] = serializedConfig
	return nil
}

func ReconcileOauthMetadata(cfg *corev1.ConfigMap) error {
	if cfg.Data == nil {
		cfg.Data = map[string]string{}
	}
	cfg.Data[OauthMetadataConfigKey] = oauthMetadata
	return nil
}

type ConfigParams struct {
	InternalAPIServerPort int
	Namespace             string
	ServiceCIDR           string
}

func generateConfig(params *ConfigParams) (string, error) {
	config := kcpv1.KubeAPIServerConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeAPIServerConfig",
			APIVersion: kcpv1.GroupVersion.String(),
		},
		APIServerArguments: map[string]kcpv1.Arguments{
			"advertise-address":   {"172.20.0.1"},
			"allow-privileged":    {"true"},
			"anonymous-auth":      {"true"},
			"api-audiences":       {"https://kubernetes.default.svc"},
			"audit-log-format":    {"json"},
			"audit-log-maxbackup": {"10"},
			"audit-log-maxsize":   {"100"},
			"audit-log-path":      {path.Join(kasWorkLogsMountPath, AuditLogFile)},
			"audit-policy-file":   {path.Join(kasAuditConfigMountPath, AuditPolicyConfigMapKey)},
			"authorization-mode":  {"Scope", "SystemMasters", "RBAC", "Node"},
			"client-ca-file":      {path.Join(kasRootCAMountPath, pki.CASignerCertMapKey)},
			"enable-admission-plugins": {
				"CertificateApproval",
				"CertificateSigning",
				"CertificateSubjectRestriction",
				"DefaultIngressClass",
				"DefaultStorageClass",
				"DefaultTolerationSeconds",
				"LimitRanger",
				"MutatingAdmissionWebhook",
				"NamespaceLifecycle",
				"NodeRestriction",
				"OwnerReferencesPermissionEnforcement",
				"PersistentVolumeClaimResize",
				"PersistentVolumeLabel",
				"PodNodeSelector",
				"PodTolerationRestriction",
				"Priority",
				"ResourceQuota",
				"RuntimeClass",
				"ServiceAccount",
				"StorageObjectInUseProtection",
				"TaintNodesByCondition",
				"ValidatingAdmissionWebhook",
				"authorization.openshift.io/RestrictSubjectBindings",
				"authorization.openshift.io/ValidateRoleBindingRestriction",
				"config.openshift.io/DenyDeleteClusterConfiguration",
				"config.openshift.io/ValidateAPIServer",
				"config.openshift.io/ValidateAuthentication",
				"config.openshift.io/ValidateConsole",
				"config.openshift.io/ValidateFeatureGate",
				"config.openshift.io/ValidateImage",
				"config.openshift.io/ValidateOAuth",
				"config.openshift.io/ValidateProject",
				"config.openshift.io/ValidateScheduler",
				"image.openshift.io/ImagePolicy",
				"network.openshift.io/ExternalIPRanger",
				"network.openshift.io/RestrictedEndpointsAdmission",
				"quota.openshift.io/ClusterResourceQuota",
				"quota.openshift.io/ValidateClusterResourceQuota",
				"route.openshift.io/IngressAdmission",
				"scheduling.openshift.io/OriginPodNodeEnvironment",
				"security.openshift.io/DefaultSecurityContextConstraints",
				"security.openshift.io/SCCExecRestrictions",
				"security.openshift.io/SecurityContextConstraint",
				"security.openshift.io/ValidateSecurityContextConstraints",
			},
			"enable-aggregator-routing":        {"true"},
			"enable-logs-handler":              {"false"},
			"enable-swagger-ui":                {"true"},
			"endpoint-reconciler-type":         {"lease"},
			"etcd-cafile":                      {path.Join(kasEtcdClientCertMountPath, etcd.ClientCAKey)},
			"etcd-certfile":                    {path.Join(kasEtcdClientCertMountPath, etcd.ClientCrtKey)},
			"etcd-keyfile":                     {path.Join(kasEtcdClientCertMountPath, etcd.ClientKeyKey)},
			"etcd-prefix":                      {"kubernetes.io"},
			"etcd-servers":                     {fmt.Sprintf("https://%s-client:%d", etcd.Cluster(params.Namespace).Name, DefaultEtcdPort)},
			"event-ttl":                        {"3h"},
			"feature-gates":                    DefaultFeatureGates,
			"goaway-chance":                    {"0"},
			"http2-max-streams-per-connection": {"2000"},
			"insecure-port":                    {"0"},
			"kubernetes-service-node-port":     {"0"},
			"max-mutating-requests-inflight":   {"1000"},
			"max-requests-inflight":            {"3000"},
			"min-request-timeout":              {"3600"},
			"proxy-client-cert-file":           {path.Join(kasAggregatorCertMountPath, corev1.TLSCertKey)},
			"proxy-client-key-file":            {path.Join(kasAggregatorCertMountPath, corev1.TLSPrivateKeyKey)},
			"requestheader-allowed-names": {
				"kube-apiserver-proxy",
				"system:kube-apiserver-proxy",
				"system:openshift-aggregator",
			},
			"requestheader-client-ca-file":       {path.Join(kasRootCAMountPath, pki.CASignerCertMapKey)},
			"requestheader-extra-headers-prefix": {"X-Remote-Extra-"},
			"requestheader-group-headers":        {"X-Remote-Group"},
			"requestheader-username-headers":     {"X-Remote-User"},
			"runtime-config":                     {"flowcontrol.apiserver.k8s.io/v1alpha1=true"},
			"service-account-issuer":             {"https://kubernetes.default.svc"},
			"service-account-lookup":             {"true"},
			"service-account-signing-key-file":   {path.Join(kasServiceAccountKeyMountPath, ServiceSignerPrivateKey)},
			"service-node-port-range":            {"30000-32767"},
			"shutdown-delay-duration":            {"70s"},
			"storage-backend":                    {"etcd3"},
			"storage-media-type":                 {"application/vnd.kubernetes.protobuf"},
			"tls-cert-file":                      {path.Join(kasServerCertMountPath, corev1.TLSCertKey)},
			"tls-private-key-file":               {path.Join(kasServerCertMountPath, corev1.TLSPrivateKeyKey)},
		},
		GenericAPIServerConfig: configv1.GenericAPIServerConfig{
			AdmissionConfig: configv1.AdmissionConfig{
				PluginConfig: map[string]configv1.AdmissionPluginConfig{
					"network.openshift.io/ExternalIPRanger": {
						Location: "",
						Configuration: runtime.RawExtension{
							Object: externalIPRangerConfig(),
						},
					},
					"network.openshift.io/RestrictedEndpointsAdmission": {
						Location: "",
						Configuration: runtime.RawExtension{
							Object: restrictedEndpointsAdmission(params.ServiceCIDR),
						},
					},
				},
			},
			ServingInfo: configv1.HTTPServingInfo{
				ServingInfo: configv1.ServingInfo{
					BindAddress: fmt.Sprintf("0.0.0.0:%d", params.InternalAPIServerPort),
					BindNetwork: "tcp4",
					CipherSuites: []string{
						"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
						"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
						"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
						"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
						"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
						"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
					},
					MinTLSVersion: "VersionTLS12",
				},
			},
			CORSAllowedOrigins: []string{
				"//127\\.0\\.0\\.1(:|$)",
				"//localhost(:|$)",
			},
		},
		AuthConfig: kcpv1.MasterAuthConfig{
			OAuthMetadataFile: path.Join(kasOauthMetadataMountPath, OauthMetadataConfigKey),
		},
		ConsolePublicURL: "https://console-openshift-console",
		ImagePolicyConfig: kcpv1.KubeAPIServerImagePolicyConfig{
			InternalRegistryHostname: "image-registry.openshift-image-registry.svc:5000",
		},
		ProjectConfig: kcpv1.KubeAPIServerProjectConfig{
			DefaultNodeSelector: "",
		},
		ServiceAccountPublicKeyFiles: []string{path.Join(kasServiceAccountKeyMountPath, ServiceSignerPublicKey)},
		ServicesSubnet:               params.ServiceCIDR,
	}
	result, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func externalIPRangerConfig() runtime.Object {
	cfg := &unstructured.Unstructured{}
	cfg.SetAPIVersion("network.openshift.io/v1")
	cfg.SetKind("ExternalIPRangerAdmissionConfig")
	unstructured.SetNestedStringSlice(cfg.Object, []string{}, "externalIPNetworkCIDRs")
	return cfg
}

func restrictedEndpointsAdmission(serviceCIDR string) runtime.Object {
	cfg := &unstructured.Unstructured{}
	cfg.SetAPIVersion("network.openshift.io/v1")
	cfg.SetKind("RestrictedEndpointsAdmissionConfig")
	unstructured.SetNestedStringSlice(cfg.Object, []string{serviceCIDR}, "restrictedCIDRs")
	return cfg
}
