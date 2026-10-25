package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mahbodhastam/falafel/internal/config"
	"github.com/mahbodhastam/falafel/internal/db"
	"github.com/mahbodhastam/falafel/internal/room"
	"github.com/mahbodhastam/falafel/internal/signaling"
)

type testEnv struct {
	server *httptest.Server
	svc    *room.Service
	hub    *signaling.Hub
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	key, err := db.SigningKey(ctx, conn, "")
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		BaseURL:           "https://falafel.test",
		MaxFileSize:       1 << 20,
		MinPasswordLength: 4,
		STUNURLs:          []string{"stun:stun.example.org:19302"},
	}
	svc := room.NewService(room.NewStore(conn), key, room.Options{
		TTL:               time.Hour,
		MaxFileSize:       cfg.MaxFileSize,
		MinPasswordLength: cfg.MinPasswordLength,
	})
	hub := signaling.NewHub(svc, logger, 50*time.Millisecond)
	spa := NewSPA(fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("<html>falafel</html>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('falafel')")},
	}, logger)
	handler := NewServer(cfg, svc, hub, logger, spa).Handler()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &testEnv{server: server, svc: svc, hub: hub}
}

func (e *testEnv) do(t *testing.T, method, path string, body string, header map[string]string) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range header {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, raw
}

func createViaAPI(t *testing.T, env *testEnv, password string) (token string, ownerSecret string) {
	t.Helper()
	body := `{"fileName":"hummus.txt","fileSize":123,"mimeType":"text/plain","password":"` + password + `"}`
	resp, raw := env.do(t, http.MethodPost, "/api/rooms", body, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", resp.StatusCode, raw)
	}
	var created struct {
		Token       string `json:"token"`
		URL         string `json:"url"`
		OwnerSecret string `json:"ownerSecret"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if !strings.HasPrefix(created.URL, "https://falafel.test/download/") {
		t.Fatalf("unexpected URL: %q", created.URL)
	}
	return created.Token, created.OwnerSecret
}

func TestCreateGetAndClose(t *testing.T) {
	env := newTestEnv(t)
	token, ownerSecret := createViaAPI(t, env, "sesame")

	resp, raw := env.do(t, http.MethodGet, "/api/rooms/"+token, "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d body = %s", resp.StatusCode, raw)
	}
	var pub room.PublicRoom
	if err := json.Unmarshal(raw, &pub); err != nil {
		t.Fatalf("decode public: %v", err)
	}
	if !pub.PasswordRequired || pub.FileName != "hummus.txt" {
		t.Fatalf("unexpected public room: %+v", pub)
	}

	resp, _ = env.do(t, http.MethodGet, "/api/rooms/"+token+"/owner", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("owner without auth = %d", resp.StatusCode)
	}
	resp, raw = env.do(t, http.MethodGet, "/api/rooms/"+token+"/owner", "", map[string]string{
		"Authorization": "Bearer " + ownerSecret,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("owner status = %d body = %s", resp.StatusCode, raw)
	}

	// sendBeacon-style close (no Authorization header).
	resp, raw = env.do(t, http.MethodPost, "/api/rooms/"+token+"/close",
		`{"ownerSecret":"`+ownerSecret+`"}`, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("close status = %d body = %s", resp.StatusCode, raw)
	}
	resp, _ = env.do(t, http.MethodGet, "/api/rooms/"+token, "", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get after close = %d", resp.StatusCode)
	}
}

func TestJoinPasswordFlow(t *testing.T) {
	env := newTestEnv(t)
	token, _ := createViaAPI(t, env, "tahini")

	resp, raw := env.do(t, http.MethodPost, "/api/rooms/"+token+"/join", `{"password":"wrong"}`, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d body = %s", resp.StatusCode, raw)
	}
	if !strings.Contains(string(raw), "wrong_password") {
		t.Fatalf("wrong password body = %s", raw)
	}

	resp, raw = env.do(t, http.MethodPost, "/api/rooms/"+token+"/join", `{"password":"tahini"}`, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("join status = %d body = %s", resp.StatusCode, raw)
	}
	var joined struct {
		Ticket string `json:"ticket"`
	}
	if err := json.Unmarshal(raw, &joined); err != nil || joined.Ticket == "" {
		t.Fatalf("join response = %s", raw)
	}
}

func TestJoinRateLimit(t *testing.T) {
	env := newTestEnv(t)
	token, _ := createViaAPI(t, env, "tahini")

	var last int
	for i := 0; i < 7; i++ {
		resp, _ := env.do(t, http.MethodPost, "/api/rooms/"+token+"/join", `{"password":"wrong"}`, nil)
		last = resp.StatusCode
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("final status = %d, want 429", last)
	}
}

func TestCreateValidation(t *testing.T) {
	env := newTestEnv(t)
	cases := []struct {
		body string
		want int
	}{
		{`{"fileName":"","fileSize":10}`, http.StatusUnprocessableEntity},
		{`{"fileName":"a.txt","fileSize":0}`, http.StatusUnprocessableEntity},
		{`{"fileName":"a.txt","fileSize":9999999999}`, http.StatusUnprocessableEntity},
		{`{"fileName":"a.txt","fileSize":10,"password":"x"}`, http.StatusUnprocessableEntity},
		{`not json`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		resp, raw := env.do(t, http.MethodPost, "/api/rooms", tc.body, nil)
		if resp.StatusCode != tc.want {
			t.Fatalf("body %s status = %d body = %s", tc.body, resp.StatusCode, raw)
		}
	}
}

func TestSPAFallback(t *testing.T) {
	env := newTestEnv(t)

	resp, raw := env.do(t, http.MethodGet, "/download/abc", "", map[string]string{"Accept": "text/html"})
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(raw), "falafel") {
		t.Fatalf("spa fallback = %d %s", resp.StatusCode, raw)
	}
	resp, _ = env.do(t, http.MethodGet, "/assets/app.js", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("asset status = %d", resp.StatusCode)
	}
	if cache := resp.Header.Get("Cache-Control"); !strings.Contains(cache, "immutable") {
		t.Fatalf("asset cache = %q", cache)
	}
	resp, raw = env.do(t, http.MethodGet, "/api/nope", "", nil)
	if resp.StatusCode != http.StatusNotFound || !strings.Contains(string(raw), "not_found") {
		t.Fatalf("unknown api = %d %s", resp.StatusCode, raw)
	}
}

func TestConfigEndpoint(t *testing.T) {
	env := newTestEnv(t)
	resp, raw := env.do(t, http.MethodGet, "/api/config", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("config status = %d", resp.StatusCode)
	}
	var cfg struct {
		IceServers []struct {
			URLs []string `json:"urls"`
		} `json:"iceServers"`
		MinPasswordLength int `json:"minPasswordLength"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if len(cfg.IceServers) != 1 || cfg.IceServers[0].URLs[0] != "stun:stun.example.org:19302" {
		t.Fatalf("ice servers = %+v", cfg.IceServers)
	}
	if cfg.MinPasswordLength != 4 {
		t.Fatalf("min password = %d", cfg.MinPasswordLength)
	}
}

func TestFailureLimiter(t *testing.T) {
	limiter := newFailureLimiter(2, time.Minute, 10*time.Second)
	now := time.Now()
	limiter.now = func() time.Time { return now }

	limiter.Fail("k")
	if ok, _ := limiter.Allow("k"); !ok {
		t.Fatal("first failure must not block")
	}
	limiter.Fail("k")
	ok, retry := limiter.Allow("k")
	if ok {
		t.Fatal("second failure must block")
	}
	if retry <= 0 || retry > 10*time.Second {
		t.Fatalf("retry = %v", retry)
	}

	now = now.Add(11 * time.Second)
	if ok, _ := limiter.Allow("k"); !ok {
		t.Fatal("block should expire")
	}
	limiter.Reset("k")
	if ok, _ := limiter.Allow("k"); !ok {
		t.Fatal("reset must allow")
	}
}

func TestMaskTokenPath(t *testing.T) {
	cases := map[string]string{
		"/api/rooms/abc123":       "/api/rooms/{token}",
		"/api/rooms/abc123/join":  "/api/rooms/{token}/join",
		"/api/rooms/abc123/owner": "/api/rooms/{token}/owner",
		"/api/ws/abc123":          "/api/ws/{token}",
		"/api/config":             "/api/config",
		"/healthz":                "/healthz",
	}
	for input, want := range cases {
		if got := maskTokenPath(input); got != want {
			t.Fatalf("maskTokenPath(%q) = %q, want %q", input, got, want)
		}
	}
}
