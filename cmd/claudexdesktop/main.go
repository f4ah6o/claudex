package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/claudex"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/util"
)

const (
	desktopPreferenceDomain = "com.anthropic.claudefordesktop"
	claudeProcessName       = "Claude"
	claudeApplicationName   = "Claude"
	configFileName          = "claudex.yaml"
	templateFileName        = "claudex.example.yaml"
)

var desktopPreferenceKeys = []string{
	"inferenceProvider",
	"inferenceGatewayBaseUrl",
	"inferenceGatewayApiKey",
	"inferenceGatewayAuthScheme",
	"inferenceCredentialKind",
	"inferenceGatewayOidc",
	"inferenceModels",
}

type preferenceValue struct {
	Present bool   `json:"present"`
	Value   string `json:"value,omitempty"`
}

type preferenceSnapshot struct {
	Values map[string]preferenceValue `json:"values"`
}

func main() {
	if err := run(); err != nil {
		_ = showMessage("ClaudexDesktop", err.Error(), false)
		fmt.Fprintf(os.Stderr, "ClaudexDesktop: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("ClaudexDesktop requires macOS")
	}

	configPath := resolveConfigPath()
	if absolutePath, errAbs := filepath.Abs(configPath); errAbs == nil {
		configPath = absolutePath
	}
	templatePath := resolveResourcePath(templateFileName)
	created, err := ensureConfig(configPath, templatePath)
	if err != nil {
		return err
	}
	pendingPath := preferenceBackupPath(configPath)
	if err = restorePendingPreferences(pendingPath); err != nil {
		return err
	}
	if created {
		_ = showMessage("ClaudexDesktop setup", fmt.Sprintf("Created %s.\n\nRun Codex login once, then open ClaudexDesktop again:\n\n%s login --config %s", configPath, resolveResourcePath("claudex-server"), configPath), true)
		return nil
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	localKey, err := localAPIKey(cfg)
	if err != nil {
		return err
	}
	if !hasAuthMaterial(cfg.AuthDir) {
		return fmt.Errorf("Codex authentication is not configured; run %s login --config %s first", resolveResourcePath("claudex-server"), configPath)
	}

	if isClaudeRunning() {
		confirmed, errConfirm := confirmRestart()
		if errConfirm != nil {
			return errConfirm
		}
		if !confirmed {
			return nil
		}
		if err = quitClaude(); err != nil {
			return err
		}
	}

	snapshot, err := capturePreferences()
	if err != nil {
		return err
	}
	if err = writePreferenceBackup(pendingPath, snapshot); err != nil {
		return err
	}
	var restoreOnce sync.Once
	restore := func() {
		restoreOnce.Do(func() {
			if errRestore := snapshot.restore(); errRestore != nil {
				logMessage("could not restore Claude Desktop preferences: " + errRestore.Error())
				return
			}
			_ = os.Remove(pendingPath)
		})
	}
	defer restore()
	if err = applyGatewayPreferences(buildBaseURL(cfg), localKey); err != nil {
		return err
	}

	signalStop := make(chan os.Signal, 1)
	signal.Notify(signalStop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalStop)
	go func() {
		<-signalStop
		restore()
		os.Exit(1)
	}()

	if _, err = startServer(cfg, configPath); err != nil {
		return err
	}
	if err = openClaude(); err != nil {
		return err
	}
	if !waitForProcess(true, 120) {
		return fmt.Errorf("Claude Desktop did not start; the standard configuration was restored")
	}
	waitForProcessExit()
	return nil
}

func resolveConfigPath() string {
	if configured := strings.TrimSpace(os.Getenv("CLAUDEX_CONFIG")); configured != "" {
		return expandHome(configured)
	}
	configRoot := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if configRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".config", "claudex", configFileName)
		}
		configRoot = filepath.Join(home, ".config")
	}
	return filepath.Join(configRoot, "claudex", configFileName)
}

func resolveResourcePath(name string) string {
	if configured := strings.TrimSpace(os.Getenv("CLAUDEX_RESOURCE_DIR")); configured != "" {
		return filepath.Join(expandHome(configured), name)
	}
	if executable, err := os.Executable(); err == nil {
		resources := filepath.Join(filepath.Dir(filepath.Dir(executable)), "Resources", name)
		if fileExists(resources) {
			return resources
		}
		for current := filepath.Dir(executable); current != filepath.Dir(current); current = filepath.Dir(current) {
			candidate := filepath.Join(current, name)
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	if workingDir, err := os.Getwd(); err == nil {
		for current := workingDir; current != filepath.Dir(current); current = filepath.Dir(current) {
			candidate := filepath.Join(current, name)
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	return name
}

func ensureConfig(path, templatePath string) (bool, error) {
	if fileExists(path) {
		return false, nil
	}
	if !fileExists(templatePath) {
		return false, fmt.Errorf("configuration template not found: %s", templatePath)
	}
	contents, err := os.ReadFile(templatePath)
	if err != nil {
		return false, fmt.Errorf("read configuration template: %w", err)
	}
	key, err := newLocalAPIKey()
	if err != nil {
		return false, err
	}
	contents = []byte(strings.ReplaceAll(string(contents), "replace-with-a-local-random-key", key))
	if err = writePrivateFile(path, contents); err != nil {
		return false, fmt.Errorf("create configuration: %w", err)
	}
	return true, nil
}

func newLocalAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate local API key: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func writePrivateFile(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".claudex-config-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err = temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err = temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func loadConfig(path string) (*config.Config, error) {
	cfg, err := config.LoadConfigOptional(path, false)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	claudex.Normalize(cfg)
	resolvedAuthDir, err := util.ResolveAuthDir(cfg.AuthDir)
	if err != nil {
		return nil, fmt.Errorf("resolve auth directory: %w", err)
	}
	cfg.AuthDir = resolvedAuthDir
	if err = claudex.ValidateServe(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func localAPIKey(cfg *config.Config) (string, error) {
	for _, key := range cfg.APIKeys {
		key = strings.TrimSpace(key)
		if key != "" && !strings.HasPrefix(strings.ToLower(key), "replace-") && !strings.HasPrefix(strings.ToLower(key), "your-api-key") {
			return key, nil
		}
	}
	return "", errors.New("Claudex configuration does not contain a usable local API key")
}

func hasAuthMaterial(path string) bool {
	path = expandHome(path)
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func buildBaseURL(cfg *config.Config) string {
	return "http://" + net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
}

func startServer(cfg *config.Config, configPath string) (*exec.Cmd, error) {
	baseURL := buildBaseURL(cfg)
	localKey, err := localAPIKey(cfg)
	if err != nil {
		return nil, err
	}
	if waitForClaudex(baseURL, localKey, 1) {
		return nil, nil
	}
	serverPath := resolveResourcePath("claudex-server")
	if !fileExists(serverPath) {
		return nil, fmt.Errorf("Claudex server binary not found: %s", serverPath)
	}
	logDir := filepath.Join(expandHome("~"), ".claudex")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, fmt.Errorf("create Claudex log directory: %w", err)
	}
	stdout, err := os.OpenFile(filepath.Join(logDir, "desktop-serve.stdout.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open Claudex stdout log: %w", err)
	}
	stderr, err := os.OpenFile(filepath.Join(logDir, "desktop-serve.stderr.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("open Claudex stderr log: %w", err)
	}
	cmd := exec.Command(serverPath, "serve", "--config", configPath)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err = cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("start Claudex server: %w", err)
	}
	_ = stdout.Close()
	_ = stderr.Close()
	if !waitForClaudex(baseURL, localKey, 60) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("Claudex server did not become ready at %s; see ~/.claudex/desktop-serve.stderr.log", baseURL)
	}
	return cmd, nil
}

func waitForClaudex(baseURL, localKey string, seconds int) bool {
	for attempt := 0; attempt < seconds*4; attempt++ {
		request, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/models", nil)
		if err == nil {
			request.Header.Set("Authorization", "Bearer "+localKey)
			request.Header.Set("Anthropic-Version", "2023-06-01")
		}
		if err == nil {
			response, err := http.DefaultClient.Do(request)
			if err == nil {
				var payload struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}
				decodeErr := json.NewDecoder(response.Body).Decode(&payload)
				_ = response.Body.Close()
				if response.StatusCode == http.StatusOK && decodeErr == nil && hasFixedModel(payload.Data) {
					return true
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func hasFixedModel(models []struct {
	ID string `json:"id"`
}) bool {
	for _, model := range models {
		if model.ID == claudex.FixedModelID {
			return true
		}
	}
	return false
}

func capturePreferences() (preferenceSnapshot, error) {
	snapshot := preferenceSnapshot{Values: make(map[string]preferenceValue, len(desktopPreferenceKeys))}
	for _, key := range desktopPreferenceKeys {
		value, present, err := readPreference(key)
		if err != nil {
			return preferenceSnapshot{}, err
		}
		snapshot.Values[key] = preferenceValue{Present: present, Value: value}
	}
	return snapshot, nil
}

func (snapshot preferenceSnapshot) restore() error {
	if len(snapshot.Values) != len(desktopPreferenceKeys) {
		return errors.New("Claude Desktop preference backup is incomplete")
	}
	for _, key := range desktopPreferenceKeys {
		value := snapshot.Values[key]
		var err error
		if value.Present {
			err = writePreference(key, value.Value)
		} else {
			err = deletePreference(key)
		}
		if err != nil {
			return fmt.Errorf("restore %s: %w", key, err)
		}
	}
	return nil
}

func preferenceBackupPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "desktop-preferences-backup.json")
}

func writePreferenceBackup(path string, snapshot preferenceSnapshot) error {
	contents, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("serialize Claude Desktop preference backup: %w", err)
	}
	if err = writePrivateFile(path, contents); err != nil {
		return fmt.Errorf("write Claude Desktop preference backup: %w", err)
	}
	return nil
}

func restorePendingPreferences(path string) error {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pending Claude Desktop preference backup: %w", err)
	}
	var snapshot preferenceSnapshot
	if err = json.Unmarshal(contents, &snapshot); err != nil {
		return fmt.Errorf("decode pending Claude Desktop preference backup: %w", err)
	}
	if err = snapshot.restore(); err != nil {
		return fmt.Errorf("restore pending Claude Desktop preferences: %w", err)
	}
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove pending Claude Desktop preference backup: %w", err)
	}
	return nil
}

func applyGatewayPreferences(baseURL, apiKey string) error {
	if err := writePreference("inferenceGatewayBaseUrl", baseURL); err != nil {
		return err
	}
	if err := writePreference("inferenceGatewayApiKey", apiKey); err != nil {
		return err
	}
	if err := writePreference("inferenceGatewayAuthScheme", "bearer"); err != nil {
		return err
	}
	for _, key := range []string{"inferenceCredentialKind", "inferenceGatewayOidc", "inferenceModels"} {
		if err := deletePreference(key); err != nil {
			return err
		}
	}
	return writePreference("inferenceProvider", "gateway")
}

func readPreference(key string) (string, bool, error) {
	output, err := exec.Command("/usr/bin/defaults", "read", desktopPreferenceDomain, key).Output()
	if err != nil {
		return "", false, nil
	}
	return strings.TrimSuffix(string(output), "\n"), true, nil
}

func writePreference(key, value string) error {
	if err := exec.Command("/usr/bin/defaults", "write", desktopPreferenceDomain, key, "-string", value).Run(); err != nil {
		return fmt.Errorf("write Claude Desktop preference %s: %w", key, err)
	}
	return nil
}

func deletePreference(key string) error {
	_ = exec.Command("/usr/bin/defaults", "delete", desktopPreferenceDomain, key).Run()
	return nil
}

func isClaudeRunning() bool {
	return exec.Command("/usr/bin/pgrep", "-x", claudeProcessName).Run() == nil
}

func confirmRestart() (bool, error) {
	output, err := exec.Command(
		"/usr/bin/osascript",
		"-e",
		`display dialog "Claude Desktop is running. Restart it in Claudex mode?" with title "ClaudexDesktop" buttons {"Cancel", "Restart"} default button "Restart"`,
	).CombinedOutput()
	if err != nil {
		return false, nil
	}
	return strings.Contains(string(output), "button returned:Restart"), nil
}

func quitClaude() error {
	if err := exec.Command("/usr/bin/osascript", "-e", `tell application "Claude" to quit`).Run(); err != nil {
		return fmt.Errorf("ask Claude Desktop to quit: %w", err)
	}
	if !waitForProcess(false, 120) {
		return errors.New("Claude Desktop did not exit after the restart request")
	}
	return nil
}

func openClaude() error {
	if err := exec.Command("/usr/bin/open", "-a", claudeApplicationName).Run(); err != nil {
		return fmt.Errorf("open Claude Desktop: %w", err)
	}
	return nil
}

func waitForProcess(running bool, attempts int) bool {
	for attempt := 0; attempt < attempts; attempt++ {
		if isClaudeRunning() == running {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func waitForProcessExit() {
	for isClaudeRunning() {
		time.Sleep(time.Second)
	}
}

func showMessage(title, message string, informational bool) error {
	message = appleScriptString(message)
	title = appleScriptString(title)
	script := fmt.Sprintf("display dialog \"%s\" with title \"%s\" buttons {\"OK\"} default button \"OK\"", message, title)
	if informational {
		script = fmt.Sprintf("display dialog \"%s\" with title \"%s\" buttons {\"OK\"} default button \"OK\"", message, title)
	}
	if err := exec.Command("/usr/bin/osascript", "-e", script).Run(); err != nil {
		return err
	}
	return nil
}

func appleScriptString(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, "\"", "\\\"")
}

func logMessage(message string) {
	path := filepath.Join(expandHome("~"), ".claudex", "desktop.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "%s %s\n", time.Now().Format(time.RFC3339), message)
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
