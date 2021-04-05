package kas

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift-hive/hypershiftlite/pkg/certs"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/pki"
)

const (
	KubeconfigKey = "kubeconfig"
)

func ReconcileServiceKubeconfigSecret(secret, ca *corev1.Secret, port int) error {
	svcURL := fmt.Sprintf("https://%s:%d", Service(secret.Namespace).Name, port)
	return reconcileSystemAdminKubeconfig(secret, ca, svcURL)
}

func ReconcileLocalhostKubeconfigSecret(secret, ca *corev1.Secret, port int) error {
	return reconcileSystemAdminKubeconfig(secret, ca, fmt.Sprintf("https://localhost:%d", port))
}

func reconcileSystemAdminKubeconfig(secret, ca *corev1.Secret, url string) error {
	if !pki.ValidCA(ca) {
		return fmt.Errorf("Invalid CA signer secret %s", ca.Name)
	}
	secret.Type = corev1.SecretTypeOpaque
	if pki.SignedSecretUpToDate(secret, ca, []string{KubeconfigKey}) {
		return nil
	}

	cfg := &certs.CertCfg{
		Subject:      pkix.Name{CommonName: "system:admin", Organization: []string{"system:masters"}},
		KeyUsages:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		Validity:     certs.ValidityOneYear,
	}
	crtBytes, keyBytes, caBytes, err := pki.SignCertificate(cfg, ca)
	if err != nil {
		return fmt.Errorf("failed to create signed cert for kubeconfig: %w", err)
	}
	kubeCfgBytes, err := generateKubeConfig(url, crtBytes, keyBytes, caBytes)
	if err != nil {
		return fmt.Errorf("failed to generate kubeconfig: %w", err)
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[KubeconfigKey] = kubeCfgBytes
	pki.AnnotateWithCA(secret, ca)
	return nil
}

func generateKubeConfig(url string, crtBytes, keyBytes, caBytes []byte) ([]byte, error) {
	kubeCfg := clientcmdapi.Config{
		Kind:       "Config",
		APIVersion: "v1",
	}
	kubeCfg.Clusters = map[string]*clientcmdapi.Cluster{
		"cluster": {
			Server:                   url,
			CertificateAuthorityData: caBytes,
		},
	}
	kubeCfg.AuthInfos = map[string]*clientcmdapi.AuthInfo{
		"admin": {
			ClientCertificateData: crtBytes,
			ClientKeyData:         keyBytes,
		},
	}
	kubeCfg.Contexts = map[string]*clientcmdapi.Context{
		"admin": {
			Cluster:   "cluster",
			AuthInfo:  "admin",
			Namespace: "default",
		},
	}
	kubeCfg.CurrentContext = "admin"
	return clientcmd.Write(kubeCfg)
}
