package resources

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	paperclipv1alpha1 "github.com/stubbi/paperclip-operator/api/v1alpha1"
)

// BuildNetworkPolicy constructs the NetworkPolicy for a Instance.
func BuildNetworkPolicy(instance *paperclipv1alpha1.Instance) *networkingv1.NetworkPolicy {
	port := servicePort(instance)
	dnsPort := intstr.FromInt32(53)

	np := &networkingv1.NetworkPolicy{
		ObjectMeta: ObjectMeta(instance, NetworkPolicyName(instance)),
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: SelectorLabels(instance),
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     Ptr(intstr.FromInt32(port)),
							Protocol: Ptr(corev1.ProtocolTCP),
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				// Allow DNS
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     &dnsPort,
							Protocol: Ptr(corev1.ProtocolUDP),
						},
						{
							Port:     &dnsPort,
							Protocol: Ptr(corev1.ProtocolTCP),
						},
					},
				},
				// Allow HTTPS outbound (for LLM API calls)
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     Ptr(intstr.FromInt32(443)),
							Protocol: Ptr(corev1.ProtocolTCP),
						},
					},
				},
			},
		},
	}

	// Allow egress to managed database if applicable
	if instance.Spec.Database.Mode == "managed" || instance.Spec.Database.Mode == "" {
		np.Spec.Egress = append(np.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: DatabaseSelectorLabels(instance),
					},
				},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Port:     Ptr(intstr.FromInt32(PostgreSQLPort)),
					Protocol: Ptr(corev1.ProtocolTCP),
				},
			},
		})
	}

	// Custom ingress CIDRs
	for _, cidr := range instance.Spec.Security.NetworkPolicy.AllowIngressCIDRs {
		np.Spec.Ingress = append(np.Spec.Ingress, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: cidr,
					},
				},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Port:     Ptr(intstr.FromInt32(port)),
					Protocol: Ptr(corev1.ProtocolTCP),
				},
			},
		})
	}

	// Custom egress CIDRs
	for _, cidr := range instance.Spec.Security.NetworkPolicy.AllowEgressCIDRs {
		np.Spec.Egress = append(np.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: cidr,
					},
				},
			},
		})
	}

	return np
}
