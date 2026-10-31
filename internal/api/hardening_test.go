package api

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mahbodhastam/falafel/internal/config"
)

func TestClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remote     string
		forwarded  string
		trustProxy bool
		want       string
	}{
		{"direct", "203.0.113.9:4444", "", false, "203.0.113.9"},
		{"forwarded header ignored without trust", "203.0.113.9:4444", "198.51.100.1", false, "203.0.113.9"},
		{"last forwarded entry wins", "10.0.0.2:80", "6.6.6.6, 198.51.100.7", true, "198.51.100.7"},
		{"spoofed leading entry ignored", "10.0.0.2:80", "1.1.1.1, 2.2.2.2, 198.51.100.7", true, "198.51.100.7"},
		{"trusted but header missing", "203.0.113.9:4444", "", true, "203.0.113.9"},
		{"ipv6 collapses to /64", "[2001:db8:1:2:aaaa:bbbb:cccc:dddd]:80", "", false, "2001:db8:1:2::"},
		{"ipv6 neighbours share a key", "[2001:db8:1:2:1111:2222:3333:4444]:80", "", false, "2001:db8:1:2::"},
		{"v4-mapped ipv6 becomes v4", "[::ffff:203.0.113.9]:80", "", false, "203.0.113.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remote
			if tc.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			if got := clientIP(r, tc.trustProxy); got != tc.want {
				t.Fatalf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTURNCredentials(t *testing.T) {
	expires := time.Unix(1_900_000_000, 0)
	username, credential := turnCredentials("s3cret", expires)
	if username != strconv.FormatInt(expires.Unix(), 10) {
		t.Fatalf("username = %q", username)
	}
	mac := hmac.New(sha1.New, []byte("s3cret"))
	mac.Write([]byte(username))
	if want := base64.StdEncoding.EncodeToString(mac.Sum(nil)); credential != want {
		t.Fatalf("credential = %q, want %q", credential, want)
	}
}

func serverWithConfig(t *testing.T, env *testEnv, cfg config.Config) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spa := NewSPA(fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}}, logger)
	return NewServer(cfg, env.svc, env.hub, logger, spa).Handler()
}

func fetchICEServers(t *testing.T, h http.Handler) []iceServer {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var body struct {
		IceServers []iceServer `json:"iceServers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	return body.IceServers
}

func TestConfigIssuesEphemeralTURNCredentials(t *testing.T) {
	env := newTestEnv(t)
	cfg := config.Config{
		TURNURLs:       []string{"turn:turn.example.org:3478"},
		TURNSecret:     "shared",
		TURNUsername:   "static-user",
		TURNCredential: "static-pass",
	}
	servers := fetchICEServers(t, serverWithConfig(t, env, cfg))
	if len(servers) != 1 {
		t.Fatalf("ice servers = %+v", servers)
	}
	got := servers[0]
	if got.Username == "static-user" || got.Credential == "static-pass" {
		t.Fatalf("static credentials leaked: %+v", got)
	}
	expiry, err := strconv.ParseInt(got.Username, 10, 64)
	if err != nil || time.Unix(expiry, 0).Before(time.Now()) {
		t.Fatalf("username %q is not a future timestamp", got.Username)
	}
}

func TestConfigFallsBackToStaticTURNCredentials(t *testing.T) {
	env := newTestEnv(t)
	cfg := config.Config{
		TURNURLs:       []string{"turn:turn.example.org:3478"},
		TURNUsername:   "static-user",
		TURNCredential: "static-pass",
	}
	servers := fetchICEServers(t, serverWithConfig(t, env, cfg))
	if len(servers) != 1 || servers[0].Username != "static-user" || servers[0].Credential != "static-pass" {
		t.Fatalf("ice servers = %+v", servers)
	}
}

func TestSecurityHeaders(t *testing.T) {
	env := newTestEnv(t)
	resp, _ := env.do(t, http.MethodGet, "/", "", nil)
	csp := resp.Header.Get("Content-Security-Policy")
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, " ws: ") || strings.Contains(csp, " wss: ") {
		t.Fatalf("CSP too permissive: %s", csp)
	}
	host := strings.TrimPrefix(env.server.URL, "http://")
	if !strings.Contains(csp, "ws://"+host) {
		t.Fatalf("CSP does not name the websocket origin %q: %s", host, csp)
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
}

func TestCSPRejectsHostInjection(t *testing.T) {
	s := &Server{cfg: config.Config{TrustProxy: true}}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-Host", "evil.example; script-src *")
	if csp := s.contentSecurityPolicy(r); strings.Contains(csp, "evil.example") {
		t.Fatalf("injected host reached CSP: %s", csp)
	}
}

func TestFailureLimiterPrunesExpiredEntries(t *testing.T) {
	limiter := newFailureLimiter(5, time.Minute, time.Minute)
	now := time.Now()
	limiter.now = func() time.Time { return now }
	for i := 0; i < 1100; i++ {
		limiter.Fail(strconv.Itoa(i))
	}
	now = now.Add(2 * time.Minute)
	limiter.Fail("fresh")
	if got := len(limiter.entries); got > 2 {
		t.Fatalf("entries after prune = %d, want the stale ones dropped", got)
	}
}
