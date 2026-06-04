package resources

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BuildNetworkPolicy constructs the NetworkPolicy for a Instance.
const managedMode = "managed"

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

	// Allow egress to OTEL collector (gRPC + HTTP) for traces and metrics
	np.Spec.Egress = append(np.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{
				Port:     Ptr(intstr.FromInt32(4317)),
				Protocol: Ptr(corev1.ProtocolTCP),
			},
			{
				Port:     Ptr(intstr.FromInt32(4318)),
				Protocol: Ptr(corev1.ProtocolTCP),
			},
		},
	})

	// Allow egress to K8s API server when cloud sandbox is enabled.
	// The server needs to create/manage sandbox pods via the K8s API.
	// An explicit rule is needed because some CNIs (k3s Flannel, Calico)
	// do not match host-network destinations with portOnly egress rules.
	if instance.Spec.Adapters.CloudSandbox != nil && instance.Spec.Adapters.CloudSandbox.Enabled {
		np.Spec.Egress = append(np.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Port:     Ptr(intstr.FromInt32(6443)),
					Protocol: Ptr(corev1.ProtocolTCP),
				},
			},
		})
	}

	// Allow Tailscale STUN (3478/udp) and WireGuard (41641/udp) egress when the
	// Tailscale sidecar is enabled. The HTTPS rule above already covers the
	// DERP relay and control-plane traffic (443/tcp).
	if instance.Spec.Tailscale.Enabled {
		np.Spec.Egress = append(np.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Port:     Ptr(intstr.FromInt32(3478)),
					Protocol: Ptr(corev1.ProtocolUDP),
				},
				{
					Port:     Ptr(intstr.FromInt32(41641)),
					Protocol: Ptr(corev1.ProtocolUDP),
				},
			},
		})
	}

	// Allow egress to database
	switch instance.Spec.Database.Mode {
	case managedMode, "":
		// Managed: restrict to the operator-created database pods
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
	case ModeExternal:
		// External: allow egress to PostgreSQL port (any destination)
		np.Spec.Egress = append(np.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
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

// BuildDatabaseNetworkPolicy constructs a NetworkPolicy restricting ingress to the managed database.
// Only allows traffic from Paperclip server pods on the PostgreSQL port.
func BuildDatabaseNetworkPolicy(instance *paperclipv1alpha1.Instance) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: ObjectMeta(instance, instance.Name+"-db"),
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: DatabaseSelectorLabels(instance),
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: SelectorLabels(instance),
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     Ptr(intstr.FromInt32(PostgreSQLPort)),
							Protocol: Ptr(corev1.ProtocolTCP),
						},
					},
				},
			},
			// Deny all egress - database does not need outbound access
			Egress: []networkingv1.NetworkPolicyEgressRule{},
		},
	}
}
