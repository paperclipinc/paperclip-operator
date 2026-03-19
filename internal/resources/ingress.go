package resources

import (
	paperclipv1alpha1 "github.com/openclaw-rocks/paperclip-operator/api/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
)

// BuildIngress constructs the Ingress for a PaperclipInstance.
func BuildIngress(instance *paperclipv1alpha1.PaperclipInstance) *networkingv1.Ingress {
	ing := instance.Spec.Networking.Ingress
	if ing == nil {
		return nil
	}

	port := servicePort(instance)
	pathType := networkingv1.PathTypePrefix

	var rules []networkingv1.IngressRule
	for _, host := range ing.Hosts {
		rules = append(rules, networkingv1.IngressRule{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: ServiceName(instance),
									Port: networkingv1.ServiceBackendPort{
										Number: port,
									},
								},
							},
						},
					},
				},
			},
		})
	}

	var tls []networkingv1.IngressTLS
	for _, t := range ing.TLS {
		tls = append(tls, networkingv1.IngressTLS{
			Hosts:      t.Hosts,
			SecretName: t.SecretName,
		})
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: ObjectMeta(instance, IngressName(instance)),
		Spec: networkingv1.IngressSpec{
			IngressClassName: ing.IngressClassName,
			Rules:            rules,
			TLS:              tls,
		},
	}

	if ing.Annotations != nil {
		ingress.Annotations = ing.Annotations
	}

	return ingress
}
