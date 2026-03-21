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
	svcURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svcName, instance.Namespace, port)

	// Follow the same flow as Paperclip's docker-onboard-smoke.sh:
	// 1. Wait for server
	// 2. Sign up admin user (creates account without admin role)
	// 3. Generate bootstrap invite via CLI
	// 4. Accept the invite with the authenticated session (promotes to admin/CEO)
	script := fmt.Sprintf(`
set -e

SERVER_URL="%s"
SVC_URL="%s"
COOKIE_JAR=$(mktemp /tmp/cookies.XXXXXX)

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

# Step 1: Sign up the admin user (or sign in if already exists)
echo "Creating admin account..."
SIGNUP_STATUS=$(curl -sS -o /tmp/signup.json -w '%%{http_code}' \
  -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "Content-Type: application/json" \
  -H "Origin: $SERVER_URL" \
  -X POST "$SVC_URL/api/auth/sign-up/email" \
  -d "{\"name\":\"%s\",\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}") || true

if echo "$SIGNUP_STATUS" | grep -q '^2'; then
  echo "Admin account created."
else
  echo "Sign-up returned HTTP $SIGNUP_STATUS, trying sign-in..."
  SIGNIN_STATUS=$(curl -sS -o /tmp/signin.json -w '%%{http_code}' \
    -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H "Content-Type: application/json" \
    -H "Origin: $SERVER_URL" \
    -X POST "$SVC_URL/api/auth/sign-in/email" \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}") || true
  if echo "$SIGNIN_STATUS" | grep -q '^2'; then
    echo "Signed in as existing admin."
  else
    echo "Could not sign up or sign in. Sign-up: $(cat /tmp/signup.json 2>/dev/null), Sign-in: $(cat /tmp/signin.json 2>/dev/null)"
    exit 1
  fi
fi

# Step 2: Check if instance is already bootstrapped
HEALTH=$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$SVC_URL/api/health" 2>/dev/null) || true
if echo "$HEALTH" | grep -q '"bootstrapStatus":"ready"'; then
  echo "Instance already bootstrapped. Nothing to do."
  rm -f "$COOKIE_JAR"
  exit 0
fi

# Step 3: Generate bootstrap invite
echo "Generating bootstrap invite..."
BOOTSTRAP_OUTPUT=$(pnpm paperclipai auth bootstrap-ceo --base-url "$SERVER_URL" 2>&1) || true
echo "$BOOTSTRAP_OUTPUT"

INVITE_TOKEN=$(echo "$BOOTSTRAP_OUTPUT" | grep -o 'pcp_bootstrap_[a-f0-9]*' | head -1)
if [ -z "$INVITE_TOKEN" ]; then
  echo "Could not extract invite token."
  rm -f "$COOKIE_JAR"
  exit 1
fi

# Step 4: Accept the invite with the authenticated session
echo "Accepting bootstrap invite..."
ACCEPT_STATUS=$(curl -sS -o /tmp/accept.json -w '%%{http_code}' \
  -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "Content-Type: application/json" \
  -H "Origin: $SERVER_URL" \
  -X POST "$SVC_URL/api/invites/$INVITE_TOKEN/accept" \
  -d '{"requestType":"human"}') || true

if echo "$ACCEPT_STATUS" | grep -q '^2'; then
  echo "Bootstrap complete. Admin user promoted to CEO."
else
  echo "Invite acceptance returned HTTP $ACCEPT_STATUS: $(cat /tmp/accept.json 2>/dev/null)"
  rm -f "$COOKIE_JAR"
  exit 1
fi

rm -f "$COOKIE_JAR"
echo "Admin bootstrap finished successfully."
`,
		baseURL,
		svcURL,
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
