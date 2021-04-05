package kas

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func ReconcileService(svc *corev1.Service, internalPort, externalPort int) error {
	if len(svc.Spec.Ports) > 0 {
		svc.Spec.Ports[0].Port = int32(externalPort)
		svc.Spec.Ports[0].TargetPort = intstr.FromInt(internalPort)
	} else {
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Port:       int32(externalPort),
				TargetPort: intstr.FromInt(internalPort),
			},
		}
	}
	svc.Spec.Selector = kasLabels
	svc.Spec.Type = corev1.ServiceTypeClusterIP
	return nil
}
