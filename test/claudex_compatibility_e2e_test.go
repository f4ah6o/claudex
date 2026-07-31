package test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/claudex"
	internalconfig "github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestClaudexProductionRoutesAndAuthentication(t *testing.T) {
	port := freeLoopbackPort(t)
	cfg := &internalconfig.Config{
		Host:    claudex.DefaultHost,
		Port:    port,
		AuthDir: t.TempDir(),
		SDKConfig: internalconfig.SDKConfig{
			APIKeys: []string{"e2e-local-key"},
		},
		WebsocketAuth: true,
	}
	claudex.Normalize(cfg)
	configPath := filepath.Join(t.TempDir(), "claudex.yaml")
	if err := os.WriteFile(configPath, []byte("host: 127.0.0.1\nport: 8317\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := claudex.NewServiceWithWatcherFactory(cfg, configPath, claudex.NoopWatcherFactory)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- service.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-runErr:
			if err != nil && err != context.Canceled {
				t.Errorf("Claudex service shutdown error: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("Claudex service did not shut down")
		}
	})

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if !waitForGateway(t, baseURL) {
		t.Fatal("Claudex service did not become ready")
	}

	request := func(method, path, body, key string) *http.Response {
		t.Helper()
		req, reqErr := http.NewRequest(method, baseURL+path, strings.NewReader(body))
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		req.Header.Set("Anthropic-Version", "2023-06-01")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		response, doErr := http.DefaultClient.Do(req)
		if doErr != nil {
			t.Fatal(doErr)
		}
		return response
	}

	response := request(http.MethodGet, "/v1/models", "", "wrong-key")
	_ = response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid auth status = %d, want 401", response.StatusCode)
	}

	response = request(http.MethodGet, "/v1/models", "", "e2e-local-key")
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("models status = %d, want 200", response.StatusCode)
	}

	response = request(http.MethodPost, "/v1/messages", `{"model":"gpt-5.5"}`, "e2e-local-key")
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("unsupported model status = %d, want 400", response.StatusCode)
	}

	response = request(http.MethodGet, "/v1/responses", "", "e2e-local-key")
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unsupported route status = %d, want 404", response.StatusCode)
	}
}

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForGateway(t *testing.T, baseURL string) bool {
	t.Helper()
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for attempt := 0; attempt < 100; attempt++ {
		request, err := http.NewRequest(http.MethodGet, baseURL+"/v1/messages", nil)
		if err == nil {
			request.Header.Set("Authorization", "Bearer e2e-local-key")
			response, doErr := client.Do(request)
			if doErr == nil {
				identity := response.Header.Get(claudex.GatewayIdentityHeader)
				_ = response.Body.Close()
				if response.StatusCode == http.StatusMethodNotAllowed && identity == claudex.GatewayIdentityValue {
					return true
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
