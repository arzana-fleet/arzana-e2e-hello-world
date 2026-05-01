// Hello-world sample app for the Arzana e2e workflow.
//
// Demonstrates the three platform-injected env vars + every kind of
// configuration the platform supports:
//
//   - PLAIN env var (FOO) — set verbatim by the user, available via os.Getenv.
//   - SECRET env var (BAR) — Key-Vault-backed; the platform resolves it
//     into env[] at deploy time. The app reads it the same way as PLAIN.
//   - SDK-only secret (STRIPE_KEY) — Key-Vault-backed; the platform never
//     emits it as an env var. The app reads it directly via azsecrets,
//     using ARZANA_KV_URL (the per-env vault URL) and the runtime's
//     managed identity.
//
// The app exposes:
//
//   - GET /healthz  — liveness, returns 200 once the SDK client is ready.
//   - GET /check-secrets — returns {foo, bar, stripe} so the e2e test
//     can verify all three resolution paths landed the values it set.
//     The endpoint deliberately echoes secret values; this is a test
//     fixture, not a security exemplar — never copy this pattern into
//     a real service.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	addr := envOrDefault("PORT", "8080")
	if addr[0] != ':' {
		addr = ":" + addr
	}

	app := &server{
		log:    log,
		kvURL:  os.Getenv("ARZANA_KV_URL"),
		env:    os.Getenv("ARZANA_ENV"),
		svc:    os.Getenv("ARZANA_SERVICE"),
		foo:    os.Getenv("FOO"),
		bar:    os.Getenv("BAR"),
		secret: "STRIPE_KEY",
	}

	// Build the SDK client lazily — at boot time the runtime's MI may
	// not have RBAC propagated yet. Resolve once on first /check-secrets
	// hit and cache.
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", app.handleHealthz)
	mux.HandleFunc("/check-secrets", app.handleCheckSecrets)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Info("hello-world starting", "addr", addr, "env", app.env, "kv_url", app.kvURL, "service", app.svc)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Error("server error", "error", err)
		os.Exit(1)
	}
}

type server struct {
	log    *slog.Logger
	kvURL  string
	env    string
	svc    string
	foo    string
	bar    string
	secret string

	once   sync.Once
	client *azsecrets.Client
}

func (s *server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *server) handleCheckSecrets(w http.ResponseWriter, r *http.Request) {
	out := map[string]string{
		"foo":      s.foo,
		"bar":      s.bar,
		"env":      s.env,
		"service":  s.svc,
		"kv_url":   s.kvURL,
		"stripe":   "",
		"sdk_err":  "",
	}
	stripe, err := s.resolveStripe(r.Context())
	if err != nil {
		out["sdk_err"] = err.Error()
	} else {
		out["stripe"] = stripe
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// resolveStripe fetches STRIPE_KEY from the per-env Key Vault using the
// runtime's managed identity. Lazily initialises the SDK client on
// first call so RBAC propagation delays at boot don't fail /healthz.
func (s *server) resolveStripe(ctx context.Context) (string, error) {
	if s.kvURL == "" {
		return "", errors.New("ARZANA_KV_URL not set — platform did not inject the per-env vault url")
	}
	s.once.Do(func() {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			s.log.Error("default credential", "error", err)
			return
		}
		client, err := azsecrets.NewClient(s.kvURL, cred, nil)
		if err != nil {
			s.log.Error("azsecrets client", "url", s.kvURL, "error", err)
			return
		}
		s.client = client
	})
	if s.client == nil {
		return "", fmt.Errorf("azsecrets client not initialised — see prior logs")
	}
	resp, err := s.client.GetSecret(ctx, s.secret, "", nil)
	if err != nil {
		return "", fmt.Errorf("get secret %q: %w", s.secret, err)
	}
	if resp.Value == nil {
		return "", fmt.Errorf("secret %q has no value", s.secret)
	}
	return *resp.Value, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
