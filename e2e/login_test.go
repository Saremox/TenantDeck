//go:build e2e

// Package e2e is the mandatory full-stack E2E suite
// (docs/spec/07-mandatory-automated-testing.md). It only builds under
// `go test -tags e2e ./e2e/...` (what `make e2e` runs), never under
// `make verify`/plain `go test ./...` - it needs the live disposable
// cluster e2e/up.sh brings up, not a fake upstream.
//
// NOT executed in this repository's own development session - see
// docs/implementation-plan.md Phase 4 "Known blockers" for the two
// independently diagnosed reasons neither kind nor a live Kubernetes
// Pod could be started in that sandbox at all.
package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// baseURL assumes `kubectl port-forward svc/tenantdeck 18080:8080 -n
// tenantdeck-e2e` (or equivalent) is already running - see
// e2e/run-tests.sh, which `make e2e` calls, for how that's arranged
// around this package.
func baseURL() string {
	if v := os.Getenv("TENANTDECK_E2E_BASE_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:18080"
}

func newClientWithCookieJar(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // follow redirects manually, one hop at a time
		},
		Timeout: 10 * time.Second,
	}
}

// loginAs performs the real HTTP authorization-code + PKCE round trip
// through mockoidc via a cookie jar - docs/spec/07 "E2E must perform HTTP
// login redirects and callback exchange with a cookie jar... Do not
// insert a session directly into the store." subject selects which
// mockoidc identity logs in (see e2e/fixtures/tenants.yaml).
func loginAs(t *testing.T, client *http.Client, subject string) {
	t.Helper()

	loginResp, err := client.Get(baseURL() + "/auth/login")
	if err != nil {
		t.Fatalf("GET /auth/login: %v", err)
	}
	loginResp.Body.Close()
	authorizeURL := loginResp.Header.Get("Location")
	if authorizeURL == "" {
		t.Fatalf("/auth/login did not redirect (status %d)", loginResp.StatusCode)
	}
	if subject != "" {
		u, err := url.Parse(authorizeURL)
		if err != nil {
			t.Fatalf("parsing authorize URL: %v", err)
		}
		q := u.Query()
		q.Set("login_hint", subject)
		u.RawQuery = q.Encode()
		authorizeURL = u.String()
	}

	authorizeResp, err := client.Get(authorizeURL)
	if err != nil {
		t.Fatalf("GET %s: %v", authorizeURL, err)
	}
	authorizeResp.Body.Close()
	callbackURL := authorizeResp.Header.Get("Location")
	if callbackURL == "" {
		t.Fatalf("mockoidc /authorize did not redirect (status %d)", authorizeResp.StatusCode)
	}

	callbackResp, err := client.Get(callbackURL)
	if err != nil {
		t.Fatalf("GET %s: %v", callbackURL, err)
	}
	callbackResp.Body.Close()
	if callbackResp.StatusCode != http.StatusFound {
		t.Fatalf("/auth/callback status = %d, want 302", callbackResp.StatusCode)
	}
}

func sessionStatus(t *testing.T, client *http.Client) map[string]any {
	t.Helper()
	resp, err := client.Get(baseURL() + "/auth/session")
	if err != nil {
		t.Fatalf("GET /auth/session: %v", err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding /auth/session: %v", err)
	}
	return body
}

func TestLogin_RealCookieJarRoundTrip_CreatesAnAuthenticatedSession(t *testing.T) {
	client := newClientWithCookieJar(t)

	before := sessionStatus(t, client)
	if before["authenticated"] != false {
		t.Fatalf("authenticated = %v before login, want false", before["authenticated"])
	}

	loginAs(t, client, "alice@tenant-a.example.com")

	after := sessionStatus(t, client)
	if after["authenticated"] != true {
		t.Fatalf("authenticated = %v after login, want true", after["authenticated"])
	}
	if after["subject"] != "alice@tenant-a.example.com" {
		t.Errorf("subject = %v, want alice@tenant-a.example.com", after["subject"])
	}
}

func TestLogout_DeniesReuseOfTheOldSessionCookie(t *testing.T) {
	client := newClientWithCookieJar(t)
	loginAs(t, client, "alice@tenant-a.example.com")

	status := sessionStatus(t, client)
	csrfToken, _ := status["csrfToken"].(string)
	if csrfToken == "" {
		t.Fatal("no csrfToken in /auth/session response")
	}

	req, err := http.NewRequest(http.MethodPost, baseURL()+"/auth/logout", nil)
	if err != nil {
		t.Fatalf("building logout request: %v", err)
	}
	req.Header.Set("Origin", baseURL())
	req.Header.Set("X-CSRF-Token", csrfToken)
	logoutResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /auth/logout: %v", err)
	}
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusNoContent {
		t.Fatalf("/auth/logout status = %d, want 204", logoutResp.StatusCode)
	}

	afterLogout := sessionStatus(t, client)
	if afterLogout["authenticated"] != false {
		t.Errorf("authenticated = %v after logout, want false (reused cookie must be denied)", afterLogout["authenticated"])
	}
}

// TestTwoReplicas_ShareSessionsAcrossPods proves the session created by
// whichever replica happened to answer the login flow is visible to
// every replica - docs/spec/06 "Test two replicas sharing sessions" -
// not just to the one that created it. `kubectl get pods` enumerates the
// live replica set; hitting the Service repeatedly (its default
// round-robin-ish load balancing) exercises more than one of them over
// enough requests without needing to target a specific Pod IP.
func TestTwoReplicas_ShareSessionsAcrossPods(t *testing.T) {
	out, err := exec.Command("kubectl", "--kubeconfig", kubeconfigPath(), "-n", "tenantdeck-e2e",
		"get", "pods", "-l", "app.kubernetes.io/name=tenantdeck", "-o", "name").Output()
	if err != nil {
		t.Fatalf("kubectl get pods: %v", err)
	}
	replicaCount := len(strings.Fields(string(out)))
	if replicaCount < 2 {
		t.Fatalf("found %d tenantdeck replica(s), want at least 2 (docs/spec/06 requires testing two)", replicaCount)
	}

	client := newClientWithCookieJar(t)
	loginAs(t, client, "bob@tenant-b.example.com")

	for i := 0; i < 10; i++ {
		status := sessionStatus(t, client)
		if status["authenticated"] != true {
			t.Fatalf("request %d: authenticated = %v, want true on every replica", i, status["authenticated"])
		}
	}
}

func kubeconfigPath() string {
	if v := os.Getenv("KUBECONFIG"); v != "" {
		return v
	}
	return "e2e/.kubeconfig"
}
