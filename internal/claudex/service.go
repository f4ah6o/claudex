package claudex

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/util"
	sdkapi "github.com/router-for-me/CLIProxyAPI/v7/sdk/api"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

// LoadConfig loads a configuration and applies the Claudex-specific defaults.
// The returned path is absolute so the service watcher observes the same file
// regardless of the caller's working directory.
func LoadConfig(path string) (*config.Config, string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, "", fmt.Errorf("configuration path is required")
	}

	resolvedPath, err := filepath.Abs(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve configuration path: %w", err)
	}
	cfg, err := config.LoadConfigOptional(resolvedPath, false)
	if err != nil {
		return nil, "", fmt.Errorf("load %s: %w", resolvedPath, err)
	}
	Normalize(cfg)
	resolvedAuthDir, err := util.ResolveAuthDir(cfg.AuthDir)
	if err != nil {
		return nil, "", fmt.Errorf("resolve auth directory: %w", err)
	}
	cfg.AuthDir = resolvedAuthDir
	return cfg, resolvedPath, nil
}

// LoadServeConfig loads and validates a configuration for the Claudex server.
func LoadServeConfig(path string) (*config.Config, string, error) {
	cfg, resolvedPath, err := LoadConfig(path)
	if err != nil {
		return nil, "", err
	}
	if err = ValidateServe(cfg); err != nil {
		return nil, "", err
	}
	return cfg, resolvedPath, nil
}

// ServerURL returns the local address used by the Claudex server.
func ServerURL(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return "http://" + net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
}

// LocalAPIKey returns the first usable client API key from the configuration.
func LocalAPIKey(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("configuration is required")
	}
	for _, key := range cfg.APIKeys {
		key = strings.TrimSpace(key)
		lower := strings.ToLower(key)
		if key != "" && !strings.HasPrefix(lower, "replace-") && !strings.HasPrefix(lower, "your-api-key") {
			return key, nil
		}
	}
	return "", fmt.Errorf("Claudex configuration does not contain a usable local API key")
}

// WaitForServer reports whether a Claudex-compatible Anthropic Messages API is
// accepting connections at the configured address. A GET request returns 405
// by design, which is the readiness response for this POST-only endpoint.
func WaitForServer(cfg *config.Config, seconds int) bool {
	if cfg == nil || seconds <= 0 {
		return false
	}
	localKey, err := LocalAPIKey(cfg)
	if err != nil {
		return false
	}
	baseURL := strings.TrimRight(ServerURL(cfg), "/")
	for attempt := 0; attempt < seconds*4; attempt++ {
		request, err := http.NewRequest(http.MethodGet, baseURL+"/v1/messages", nil)
		if err == nil {
			request.Header.Set("Authorization", "Bearer "+localKey)
			request.Header.Set("Anthropic-Version", "2023-06-01")
			response, err := http.DefaultClient.Do(request)
			if err == nil {
				_ = response.Body.Close()
				if response.StatusCode == http.StatusMethodNotAllowed {
					return true
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

// NewService builds the Claudex-restricted proxy service shared by the CLI and
// the desktop-bundled server.
func NewService(cfg *config.Config, configPath string) (*cliproxy.Service, error) {
	if err := ValidateServe(cfg); err != nil {
		return nil, err
	}
	service, err := cliproxy.NewBuilder().
		WithConfig(cfg).
		WithConfigPath(configPath).
		WithServerOptions(
			sdkapi.WithMiddleware(Middleware(cfg)),
			sdkapi.WithAnthropicModelsHandler(FixedModelsHandler()),
		).
		Build()
	if err != nil {
		return nil, fmt.Errorf("build service: %w", err)
	}
	return service, nil
}
