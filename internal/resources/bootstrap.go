package resources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BootstrapHashAnnotation records a content hash of the operator-controlled
// inputs to the bootstrap Job. Because a Job's pod template is immutable after
// creation, the reconciler never patches an existing Job in place; instead it
// compares this annotation and, only when it differs, deletes and recreates the
// Job. A steady-state reconcile sees an unchanged hash and is a no-op.
const BootstrapHashAnnotation = "paperclip.ai/bootstrap-spec-hash"

// BootstrapJobLabel is a per-Job unique pod label. It is part of the Job's
// explicit selector (manualSelector) so the Job controller can never adopt a
// pod left over from a previous bootstrap Job of the same name. Adopting a
// stale/orphaned pod is what made the Job controller reap its own pod ~1s after
// start and then report BackoffLimitExceeded (issue #83).
const BootstrapJobLabel = "paperclip.ai/bootstrap-job"

// sanitizeJSONString escapes special characters for safe embedding in a JSON string literal
// inside a shell script. Prevents JSON injection via user-controlled CRD fields.
func sanitizeJSONString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

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

	adminName := sanitizeJSONString(admin.Name)
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
  -X POST "$SERVER_URL/api/auth/sign-up/email" \
  -d "{\"name\":\"%s\",\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}") || true

if echo "$SIGNUP_STATUS" | grep -q '^2'; then
  echo "Admin account created."
else
  echo "Sign-up returned HTTP $SIGNUP_STATUS, trying sign-in..."
  SIGNIN_STATUS=$(curl -sS -o /tmp/signin.json -w '%%{http_code}' \
    -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H "Content-Type: application/json" \
    -H "Origin: $SERVER_URL" \
    -X POST "$SERVER_URL/api/auth/sign-in/email" \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}") || true
  if echo "$SIGNIN_STATUS" | grep -q '^2'; then
    echo "Signed in as existing admin."
  else
    echo "Could not sign up or sign in. Sign-up: $(cat /tmp/signup.json 2>/dev/null), Sign-in: $(cat /tmp/signin.json 2>/dev/null)"
    exit 1
  fi
fi

# Step 2: Check if instance is already bootstrapped
# Use /api/health/details (authenticated) which includes bootstrapStatus;
# the plain /api/health endpoint does not return this field.
HEALTH=$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$SERVER_URL/api/health/details" 2>/dev/null) || true
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
  -X POST "$SERVER_URL/api/invites/$INVITE_TOKEN/accept" \
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

	jobName := BootstrapJobName(instance)

	// Pod template labels: the standard component labels plus a per-Job unique
	// label. The unique label is also the Job's explicit selector
	// (manualSelector=true) so this Job's pods can never be confused with — or
	// adopted from — a previous bootstrap Job of the same name. Without an
	// explicit, unique selector the Job controller could adopt a leftover/
	// orphaned pod and then delete it, killing the running bootstrap pod ~1s
	// after start and tripping BackoffLimitExceeded (issue #83).
	podLabels := LabelsWithComponent(instance, "bootstrap")
	podLabels[BootstrapJobLabel] = jobName

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: instance.Namespace,
			Labels:    LabelsWithComponent(instance, "bootstrap"),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			ManualSelector:          Ptr(true),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{BootstrapJobLabel: jobName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyOnFailure,
					AutomountServiceAccountToken: Ptr(false),
					NodeSelector:                 instance.Spec.Availability.NodeSelector,
					Tolerations:                  instance.Spec.Availability.Tolerations,
					ImagePullSecrets:             instance.Spec.Image.PullSecrets,
					// spec.security.podSecurityContext must reach this Job just
					// like it reaches the server pod template. Hard-coding the
					// UID/GID 1000 default here made the Job fail admission
					// under OpenShift's restricted-v2 SCC, which only admits the
					// namespace's dynamically allocated UID/GID range, so the
					// Job never created a pod and the Instance stayed in
					// Provisioning forever (issue #111).
					SecurityContext: paperclipPodSecurityContext(instance),
					Containers: []corev1.Container{
						{
							Name:            "bootstrap",
							Image:           image,
							ImagePullPolicy: imagePullPolicy(instance),
							Command:         []string{"/bin/sh", "-c"},
							Args:            []string{script},
							SecurityContext: paperclipContainerSecurityContext(instance),
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

	// Stamp a content hash of the operator-controlled inputs. The reconciler
	// uses this to decide whether an existing Job is up to date (no-op) or must
	// be replaced (delete + recreate); it never patches the immutable template
	// in place.
	if job.Annotations == nil {
		job.Annotations = map[string]string{}
	}
	job.Annotations[BootstrapHashAnnotation] = bootstrapSpecHash(
		image, script, admin, job.Spec.Template.Spec.SecurityContext)

	return job
}

// bootstrapSpecHash returns a stable hash over the operator-controlled inputs to
// the bootstrap Job. It intentionally excludes server-defaulted/mutable fields
// so a steady-state reconcile produces an identical hash and is a no-op.
func bootstrapSpecHash(
	image, script string,
	admin *paperclipv1alpha1.AdminUserSpec,
	podSecurityContext *corev1.PodSecurityContext,
) string {
	// The script already embeds the resolved admin name, service/base URLs and
	// the JSON sign-up payload, so it captures most config drift. Include the
	// image, admin email and the password secret reference explicitly so a
	// change to credentials forces a fresh Job.
	//
	// The pod security context is hashed too: a Job's pod template is immutable,
	// so without it an Instance whose spec.security.podSecurityContext changes
	// would keep the stale (and, on OpenShift, unschedulable) Job forever
	// (issue #111). encoding/json emits struct fields in declaration order, so
	// this serialization is stable across reconciles.
	securityContextJSON, err := json.Marshal(podSecurityContext)
	if err != nil {
		// PodSecurityContext is a plain API struct and cannot fail to marshal;
		// fall back to a constant rather than panicking in a builder.
		securityContextJSON = []byte("unmarshalable")
	}
	payload := fmt.Sprintf("image=%s\x00script=%s\x00email=%s\x00secret=%s/%s\x00podSecurityContext=%s\x00",
		image, script, admin.Email,
		admin.PasswordSecretRef.Name, admin.PasswordSecretRef.Key,
		securityContextJSON)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
