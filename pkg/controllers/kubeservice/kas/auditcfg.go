package kas

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	AuditPolicyConfigMapKey = "policy.yaml"
)

const defaultAuditConfig = `
apiVersion: audit.k8s.io/v1beta1
kind: Policy
omitStages:
- RequestReceived
rules:
- level: None
  resources:
  - group: ''
    resources:
    - events
- level: None
  resources:
  - group: oauth.openshift.io
    resources:
    - oauthaccesstokens
    - oauthauthorizetokens
- level: None
  nonResourceURLs:
  - "/api*"
  - "/version"
  - "/healthz"
  - "/readyz"
  userGroups:
  - system:authenticated
  - system:unauthenticated
- level: Metadata
  omitStages:
  - RequestReceived
`

func ReconcileAuditConfig(auditCfgMap *corev1.ConfigMap) error {
	if auditCfgMap.Data == nil {
		auditCfgMap.Data = map[string]string{}
	}
	auditCfgMap.Data[AuditPolicyConfigMapKey] = defaultAuditConfig
	return nil
}
