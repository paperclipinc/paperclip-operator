package resources

import (
	networkingv1 "k8s.io/api/networking/v1"

	paperclipv1alpha1 "github.com/stubbi/paperclip-operator/api/v1alpha1"
)

// BuildIngress constructs the Ingress for a Instance.
func BuildIngress(instance *paperclipv1alpha1.Instance) *networkingv1.Ingress {
	ing := instance.Spec.Networking.Ingress
	if ing == nil {
		return nil
	}

	port := servicePort(instance)
	pathType := networkingv1.PathTypePrefix

	rules := make([]networkingv1.IngressRule, 0, len(ing.Hosts))
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

	tls := make([]networkingv1.IngressTLS, 0, len(ing.TLS))
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
