package kas

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"

	"github.com/openshift-hive/hypershiftlite/pkg/certs"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/pki"
)

const (
	// Service signer secret keys
	ServiceSignerPrivateKey = "service-account.key"
	ServiceSignerPublicKey  = "service-account.pub"
)

func ReconcileServerCertSecret(secret, ca *corev1.Secret, serviceCIDR string) error {
	if !pki.ValidCA(ca) {
		return fmt.Errorf("Invalid CA signer secret %s", ca.Name)
	}
	svc := Service(secret.Namespace)
	secret.Type = corev1.SecretTypeTLS
	expectedKeys := []string{corev1.TLSCertKey, corev1.TLSPrivateKeyKey}
	if !pki.SignedSecretUpToDate(secret, ca, expectedKeys) {
		serviceName := svc.Name
		serviceNamespace := svc.Namespace
		_, serviceIPNet, err := net.ParseCIDR(serviceCIDR)
		if err != nil {
			return fmt.Errorf("cannot parse service CIDR: %w", err)
		}
		serviceIP := firstIP(serviceIPNet)
		dnsNames := []string{
			"localhost",
			"kubernetes",
			"kubernetes.default.svc",
			"kubernetes.default.svc.cluster.local",
			serviceName,
			fmt.Sprintf("%s.%s.svc", serviceName, serviceNamespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, serviceNamespace),
		}
		apiServerIPs := []net.IP{
			net.ParseIP("127.0.0.1"),
			serviceIP,
		}
		cfg := &certs.CertCfg{
			Subject:      pkix.Name{CommonName: "kubernetes", Organization: []string{"kubernetes"}},
			KeyUsages:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			Validity:     certs.ValidityOneYear,
			DNSNames:     dnsNames,
			IPAddresses:  apiServerIPs,
		}
		crtBytes, keyBytes, _, err := pki.SignCertificate(cfg, ca)
		if err != nil {
			return fmt.Errorf("failed to sign secret: %w", err)
		}
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Data[corev1.TLSCertKey] = crtBytes
		secret.Data[corev1.TLSPrivateKeyKey] = keyBytes
		pki.AnnotateWithCA(secret, ca)
	}
	return nil
}

func ReconcileAggregatorCertSecret(secret, ca *corev1.Secret) error {
	if !pki.ValidCA(ca) {
		return fmt.Errorf("Invalid CA signer secret %s", ca.Name)
	}
	secret.Type = corev1.SecretTypeTLS
	expectedKeys := []string{corev1.TLSCertKey, corev1.TLSPrivateKeyKey}
	if !pki.SignedSecretUpToDate(secret, ca, expectedKeys) {
		cfg := &certs.CertCfg{
			Subject:      pkix.Name{CommonName: "system:openshift-aggregator", Organization: []string{"kubernetes"}},
			KeyUsages:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			Validity:     certs.ValidityOneYear,
		}
		crtBytes, keyBytes, _, err := pki.SignCertificate(cfg, ca)
		if err != nil {
			return fmt.Errorf("failed to sign secret: %w", err)
		}
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Data[corev1.TLSCertKey] = crtBytes
		secret.Data[corev1.TLSPrivateKeyKey] = keyBytes
		pki.AnnotateWithCA(secret, ca)
	}
	return nil
}

func ReconcileServiceAccountSigningKeySecret(secret *corev1.Secret) error {
	secret.Type = corev1.SecretTypeOpaque
	expectedKeys := []string{ServiceSignerPrivateKey, ServiceSignerPublicKey}
	if !pki.SecretUpToDate(secret, expectedKeys) {
		key, err := certs.PrivateKey()
		if err != nil {
			return fmt.Errorf("failed generating a private key: %w", err)
		}
		keyBytes := certs.PrivateKeyToPem(key)
		publicKeyBytes, err := certs.PublicKeyToPem(&key.PublicKey)
		if err != nil {
			return fmt.Errorf("failed to generate public key from private key: %w", err)
		}
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Data[ServiceSignerPrivateKey] = keyBytes
		secret.Data[ServiceSignerPublicKey] = publicKeyBytes
	}
	return nil
}

func nextIP(ip net.IP) net.IP {
	nextIP := net.IP(make([]byte, len(ip)))
	copy(nextIP, ip)
	for j := len(nextIP) - 1; j >= 0; j-- {
		nextIP[j]++
		if nextIP[j] > 0 {
			break
		}
	}
	return nextIP
}

func firstIP(network *net.IPNet) net.IP {
	return nextIP(network.IP)
}
