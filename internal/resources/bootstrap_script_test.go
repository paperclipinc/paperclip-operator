package resources

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// The bootstrap Job's behavior lives in a shell script rendered by
// BuildBootstrapJob, so asserting on the Go struct alone cannot catch a broken
// script. Issue #112 was exactly that: the idempotency pre-check queried
// /api/health/details, a route the shipping server 404s, so the "already
// bootstrapped, nothing to do" branch was unreachable and every long-lived,
// already-bootstrapped Instance failed its bootstrap Job forever on a ~65 minute
// loop. These tests execute the real rendered script against a stub server.

// clusterURLPattern matches the in-cluster Service URLs baked into the script,
// so a test can point them at an httptest server.
var clusterURLPattern = regexp.MustCompile(`http://[^"\s]*\.svc\.cluster\.local:\d+`)

type scriptResult struct {
	exitCode  int
	output    string
	cliCalled bool
}

// runBootstrapScript renders the bootstrap Job script for instance, retargets it
// at serverURL, and executes it with a stub `pnpm` on PATH that prints
// cliOutput.
func runBootstrapScript(t *testing.T, instance *paperclipv1alpha1.Instance, serverURL, cliOutput string) scriptResult {
	t.Helper()

	sh, lookErr := exec.LookPath("sh")
	if lookErr != nil {
		t.Skip("sh not available")
	}
	if _, curlErr := exec.LookPath("curl"); curlErr != nil {
		t.Skip("curl not available")
	}

	job := BuildBootstrapJob(instance)
	if job == nil {
		t.Fatal("BuildBootstrapJob returned nil")
	}
	script := job.Spec.Template.Spec.Containers[0].Args[0]
	script = clusterURLPattern.ReplaceAllString(script, serverURL)
	script = strings.ReplaceAll(script, instance.Spec.Deployment.PublicURL, serverURL)

	binDir := t.TempDir()
	markerPath := filepath.Join(binDir, "cli-was-called")
	stub := "#!/bin/sh\ntouch " + markerPath + "\ncat <<'PNPM_STUB_EOF'\n" + cliOutput + "\nPNPM_STUB_EOF\n"
	if writeErr := os.WriteFile(filepath.Join(binDir, "pnpm"), []byte(stub), 0o755); writeErr != nil {
		t.Fatalf("writing pnpm stub: %v", writeErr)
	}

	scriptPath := filepath.Join(t.TempDir(), "bootstrap.sh")
	if writeErr := os.WriteFile(scriptPath, []byte(script), 0o644); writeErr != nil {
		t.Fatalf("writing script: %v", writeErr)
	}

	cmd := exec.CommandContext(t.Context(), sh, scriptPath) // #nosec G204 -- test-only, path is from t.TempDir()
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ADMIN_EMAIL=admin@test.com",
		"ADMIN_PASSWORD=hunter2",
	)
	out, err := cmd.CombinedOutput()

	res := scriptResult{output: string(out)}
	if err != nil {
		var exitErr *exec.ExitError
		if !asExitError(err, &exitErr) {
			t.Fatalf("running script: %v\n%s", err, out)
		}
		res.exitCode = exitErr.ExitCode()
	}
	if _, statErr := os.Stat(markerPath); statErr == nil {
		res.cliCalled = true
	}
	return res
}

func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}

// bootstrapStubServer serves the endpoints the bootstrap script talks to.
// healthBody/healthCode and detailsBody/detailsCode control the two health
// probes independently.
type bootstrapStubServer struct {
	healthCode   int
	healthBody   string
	detailsCode  int
	detailsBody  string
	signUpCode   int
	signInCode   int
	acceptCode   int
	healthHits   []string
	acceptedPath string
}

func (s *bootstrapStubServer) start(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		s.healthHits = append(s.healthHits, "/api/health")
		w.WriteHeader(s.healthCode)
		_, _ = w.Write([]byte(s.healthBody))
	})
	mux.HandleFunc("/api/health/details", func(w http.ResponseWriter, _ *http.Request) {
		s.healthHits = append(s.healthHits, "/api/health/details")
		w.WriteHeader(s.detailsCode)
		_, _ = w.Write([]byte(s.detailsBody))
	})
	mux.HandleFunc("/api/auth/sign-up/email", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(s.signUpCode)
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/api/auth/sign-in/email", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(s.signInCode)
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/api/invites/", func(w http.ResponseWriter, req *http.Request) {
		s.acceptedPath = req.URL.Path
		w.WriteHeader(s.acceptCode)
		_, _ = w.Write([]byte(`{}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func newBootstrapScriptInstance(serverURL string) *paperclipv1alpha1.Instance {
	instance := newTestInstance("script-test")
	instance.Spec.Deployment.PublicURL = serverURL
	instance.Spec.Auth.AdminUser = &paperclipv1alpha1.AdminUserSpec{
		Email: "admin@test.com",
		PasswordSecretRef: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "admin-secret"},
			Key:                  "password",
		},
	}
	return instance
}

const readyHealth = `{"status":"ok","deploymentMode":"authenticated","deploymentExposure":"private",` +
	`"bootstrapStatus":"ready","bootstrapInviteActive":false}`

const pendingHealth = `{"status":"ok","deploymentMode":"authenticated","deploymentExposure":"private",` +
	`"bootstrapStatus":"bootstrap_pending","bootstrapInviteActive":false}`

const notFoundBody = `{"error":"API route not found"}`

const adminExistsCLIOutput = "> node cli/node_modules/tsx/dist/cli.mjs cli/src/index.ts \"auth\" \"bootstrap-ceo\"\n" +
	"|  Instance already has an admin user. Use --force to generate a new bootstrap invite."

func TestBootstrapScriptShortCircuitsOnHealth(t *testing.T) {
	// The shipping server: /api/health carries bootstrapStatus,
	// /api/health/details does not exist. This is the exact shape from #112.
	stub := &bootstrapStubServer{
		healthCode:  http.StatusOK,
		healthBody:  readyHealth,
		detailsCode: http.StatusNotFound,
		detailsBody: notFoundBody,
		signUpCode:  http.StatusUnprocessableEntity,
		signInCode:  http.StatusOK,
	}
	url := stub.start(t)

	res := runBootstrapScript(t, newBootstrapScriptInstance(url), url, adminExistsCLIOutput)

	if res.exitCode != 0 {
		t.Errorf("expected exit 0 on an already-bootstrapped instance, got %d\n%s", res.exitCode, res.output)
	}
	if !strings.Contains(res.output, "Instance already bootstrapped. Nothing to do.") {
		t.Errorf("expected the short-circuit branch to be reached, output was:\n%s", res.output)
	}
	if res.cliCalled {
		t.Errorf("bootstrap-ceo must not run once health reports bootstrapStatus=ready\n%s", res.output)
	}
	if len(stub.healthHits) != 1 || stub.healthHits[0] != "/api/health" {
		t.Errorf("expected exactly one probe of /api/health, got %v", stub.healthHits)
	}
}

func TestBootstrapScriptFallsBackWhenHealthEndpointMoves(t *testing.T) {
	// A future rename must not wedge bootstrap again: if /api/health stops
	// reporting bootstrapStatus, the script keeps probing.
	t.Run("primary 404s", func(t *testing.T) {
		stub := &bootstrapStubServer{
			healthCode:  http.StatusNotFound,
			healthBody:  notFoundBody,
			detailsCode: http.StatusOK,
			detailsBody: readyHealth,
			signUpCode:  http.StatusUnprocessableEntity,
			signInCode:  http.StatusOK,
		}
		url := stub.start(t)

		res := runBootstrapScript(t, newBootstrapScriptInstance(url), url, adminExistsCLIOutput)

		if res.exitCode != 0 {
			t.Errorf("expected exit 0, got %d\n%s", res.exitCode, res.output)
		}
		if !strings.Contains(res.output, "Instance already bootstrapped. Nothing to do.") {
			t.Errorf("expected the fallback probe to short-circuit, output was:\n%s", res.output)
		}
	})

	t.Run("primary 200s without the field", func(t *testing.T) {
		stub := &bootstrapStubServer{
			healthCode:  http.StatusOK,
			healthBody:  `{"status":"ok"}`,
			detailsCode: http.StatusOK,
			detailsBody: readyHealth,
			signUpCode:  http.StatusUnprocessableEntity,
			signInCode:  http.StatusOK,
		}
		url := stub.start(t)

		res := runBootstrapScript(t, newBootstrapScriptInstance(url), url, adminExistsCLIOutput)

		if res.exitCode != 0 {
			t.Errorf("expected exit 0, got %d\n%s", res.exitCode, res.output)
		}
		if !strings.Contains(res.output, "Instance already bootstrapped. Nothing to do.") {
			t.Errorf("expected the fallback probe to short-circuit, output was:\n%s", res.output)
		}
	})
}

func TestBootstrapScriptTreatsExistingAdminAsSuccess(t *testing.T) {
	// Second line of defense: even with every health probe broken, an
	// already-bootstrapped instance must not fail the Job. bootstrap-ceo
	// refusing because an admin exists is a correct idempotent outcome.
	stub := &bootstrapStubServer{
		healthCode:  http.StatusNotFound,
		healthBody:  notFoundBody,
		detailsCode: http.StatusNotFound,
		detailsBody: notFoundBody,
		signUpCode:  http.StatusUnprocessableEntity,
		signInCode:  http.StatusOK,
	}
	url := stub.start(t)

	res := runBootstrapScript(t, newBootstrapScriptInstance(url), url, adminExistsCLIOutput)

	if res.exitCode != 0 {
		t.Errorf("expected exit 0 when bootstrap-ceo reports an existing admin, got %d\n%s", res.exitCode, res.output)
	}
	if !strings.Contains(res.output, "Instance already has an admin user. Nothing to do.") {
		t.Errorf("expected the idempotent-success branch, output was:\n%s", res.output)
	}
	if !res.cliCalled {
		t.Error("expected bootstrap-ceo to run when no health probe reports bootstrapStatus")
	}
}

func TestBootstrapScriptBootstrapsFreshInstance(t *testing.T) {
	stub := &bootstrapStubServer{
		healthCode:  http.StatusOK,
		healthBody:  pendingHealth,
		detailsCode: http.StatusNotFound,
		detailsBody: notFoundBody,
		signUpCode:  http.StatusOK,
		signInCode:  http.StatusOK,
		acceptCode:  http.StatusOK,
	}
	url := stub.start(t)

	res := runBootstrapScript(t, newBootstrapScriptInstance(url), url,
		"Bootstrap invite created: pcp_bootstrap_abc123def456")

	if res.exitCode != 0 {
		t.Errorf("expected exit 0 for a fresh instance, got %d\n%s", res.exitCode, res.output)
	}
	if !strings.Contains(res.output, "Bootstrap complete. Admin user promoted to CEO.") {
		t.Errorf("expected a full bootstrap, output was:\n%s", res.output)
	}
	if stub.acceptedPath != "/api/invites/pcp_bootstrap_abc123def456/accept" {
		t.Errorf("unexpected invite accept path %q", stub.acceptedPath)
	}
}

func TestBootstrapScriptStillFailsOnRealErrors(t *testing.T) {
	// The idempotent-success branch must be narrow: any other reason for a
	// missing invite token is still a genuine failure.
	stub := &bootstrapStubServer{
		healthCode:  http.StatusNotFound,
		healthBody:  notFoundBody,
		detailsCode: http.StatusNotFound,
		detailsBody: notFoundBody,
		signUpCode:  http.StatusUnprocessableEntity,
		signInCode:  http.StatusOK,
	}
	url := stub.start(t)

	res := runBootstrapScript(t, newBootstrapScriptInstance(url), url,
		"Could not resolve database connection for bootstrap.")

	if res.exitCode == 0 {
		t.Errorf("expected a nonzero exit when bootstrap genuinely fails, output was:\n%s", res.output)
	}
	if !strings.Contains(res.output, "Could not extract invite token.") {
		t.Errorf("expected the failure branch, output was:\n%s", res.output)
	}
}
