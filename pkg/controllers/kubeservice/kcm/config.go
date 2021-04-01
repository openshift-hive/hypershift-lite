package kcm

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kcpv1 "github.com/openshift/api/kubecontrolplane/v1"
)

const (
	KubeControllerManagerConfigKey = "config.json"
)

func ReconcileConfig(config *corev1.ConfigMap) error {
	if config.Data == nil {
		config.Data = map[string]string{}
	}
	serializedConfig, err := generateConfig()
	if err != nil {
		return fmt.Errorf("failed to create apiserver config: %w", err)
	}
	config.Data[KubeControllerManagerConfigKey] = serializedConfig
	return nil
}

func generateConfig() (string, error) {
	config := kcpv1.KubeControllerManagerConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeControllerManagerConfig",
			APIVersion: kcpv1.GroupVersion.String(),
		},
		ExtendedArguments: map[string]kcpv1.Arguments{},
		ServiceServingCert: kcpv1.ServiceServingCert{
			CertFile: fmt.Sprintf(""),
		},
	}
	b, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
