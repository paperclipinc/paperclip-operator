package resources

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BootstrapJobName returns the bootstrap Job name for an Instance.
func BootstrapJobName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name + "-bootstrap"
}

// BuildBootstrapJob constructs a Job that creates the initial admin user.
// The Job waits for the Paperclip server to be healthy, runs bootstrap-ceo
// to generate an invite token, then calls the sign-up API to create the admin.
func BuildBootstrapJob(instance *paperclipv1alpha1.Instance) *batchv1.Job {
	admin := instance.Spec.Auth.AdminUser
	if admin == nil {
		return nil
	}

	image := containerImage(instance)
	port := servicePort(instance)
	svcName := ServiceName(instance)

	adminName := admin.Name
	if adminName == "" {
		adminName = "Admin"
	}

	baseURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svcName, instance.Namespace, port)
	if instance.Spec.Deployment.PublicURL != "" {
		baseURL = instance.Spec.Deployment.PublicURL
	}

	// Script:
	// 1. Wait for the server to accept connections
	// 2. Run bootstrap-ceo to get the invite token
	// 3. Call the sign-up API with the token and admin credentials
	script := fmt.Sprintf(`
set -e

SERVER_URL="%s"
SVC_URL="http://%s.%s.svc.cluster.local:%d"

echo "Waiting for Paperclip server..."
for i in $(seq 1 60); do
  HTTP_CODE=$(curl -s -o /dev/null -w '%%{http_code}' "$SVC_URL/") || true
  if [ "$HTTP_CODE" != "000" ] && [ -n "$HTTP_CODE" ]; then
    echo "Server is ready (HTTP $HTTP_CODE)."
    break
  fi
  echo "Waiting... ($i/60)"
  sleep 5
done

echo "Running bootstrap-ceo..."
BOOTSTRAP_OUTPUT=$(pnpm paperclipai auth bootstrap-ceo --base-url "$SERVER_URL" 2>&1) || true
echo "$BOOTSTRAP_OUTPUT"

# Extract the invite token from the output
INVITE_TOKEN=$(echo "$BOOTSTRAP_OUTPUT" | grep -o 'pcp_bootstrap_[a-f0-9]*' | head -1)

if [ -z "$INVITE_TOKEN" ]; then
  if echo "$BOOTSTRAP_OUTPUT" | grep -qi "already exists\|already been"; then
    echo "Admin user already exists. Nothing to do."
    exit 0
  fi
  echo "Could not extract invite token."
  exit 1
fi

echo "Creating admin user with invite token..."
RESPONSE=$(curl -s -X POST "$SERVER_URL/api/auth/sign-up/email" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\",\"name\":\"%s\",\"inviteToken\":\"$INVITE_TOKEN\"}") || true

if echo "$RESPONSE" | grep -q '"user"'; then
  echo "Admin user created successfully."
elif echo "$RESPONSE" | grep -qi "already exists\|duplicate"; then
  echo "Admin user already exists."
  exit 0
else
  echo "Sign-up response: $RESPONSE"
  exit 1
fi
`,
		baseURL,
		svcName, instance.Namespace, port,
		adminName,
	)

	backoffLimit := int32(3)
	ttl := int32(3600) // Clean up completed job after 1 hour

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BootstrapJobName(instance),
			Namespace: instance.Namespace,
			Labels:    LabelsWithComponent(instance, "bootstrap"),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: LabelsWithComponent(instance, "bootstrap"),
				},
				Spec: corev1.PodSpec{
					RestartPolicy:    corev1.RestartPolicyOnFailure,
					NodeSelector:     instance.Spec.Availability.NodeSelector,
					Tolerations:      instance.Spec.Availability.Tolerations,
					ImagePullSecrets: instance.Spec.Image.PullSecrets,
					Containers: []corev1.Container{
						{
							Name:            "bootstrap",
							Image:           image,
							ImagePullPolicy: imagePullPolicy(instance),
							Command:         []string{"/bin/sh", "-c"},
							Args:            []string{script},
							Env: append(buildEnvVars(instance),
								corev1.EnvVar{
									Name:  "ADMIN_EMAIL",
									Value: admin.Email,
								},
								corev1.EnvVar{
									Name: "ADMIN_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &admin.PasswordSecretRef,
									},
								},
							),
							EnvFrom:      instance.Spec.EnvFrom,
							VolumeMounts: buildVolumeMounts(instance),
						},
					},
					Volumes: buildVolumes(instance),
				},
			},
		},
	}
}
