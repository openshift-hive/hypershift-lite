package kcm

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/openshift-hive/hypershiftlite/pkg/certs"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/pki"
)

const (
	SignerSecretCertKey = "ca.crt"
	SignerSecretKeyKey  = "ca.key"
)

func ReconcileClusterSignerSecret(secret, ca *corev1.Secret) error {
	if !pki.ValidCA(ca) {
		return fmt.Errorf("Invalid CA signer secret %s", ca.Name)
	}
	secret.Type = corev1.SecretTypeOpaque
	expectedKeys := []string{SignerSecretCertKey, SignerSecretKeyKey}
	if !pki.SignedSecretUpToDate(secret, ca, expectedKeys) {
		cfg := &certs.CertCfg{
			Subject:      pkix.Name{CommonName: "cluster-signer", Organization: []string{"openshift"}},
			KeyUsages:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			Validity:     certs.ValidityTenYears,
			IsCA:         true,
		}
		crtBytes, keyBytes, _, err := pki.SignCertificate(cfg, ca)
		if err != nil {
			return fmt.Errorf("failed to sign secret: %w", err)
		}
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Data[SignerSecretCertKey] = crtBytes
		secret.Data[SignerSecretKeyKey] = keyBytes
		pki.AnnotateWithCA(secret, ca)
	}
	return nil
}
