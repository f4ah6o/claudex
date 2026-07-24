package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/claudex"
)

const (
	configFileName       = "claudex.yaml"
	backupFileName       = "linux-desktop-config-backup.json"
	defaultDesktopBinary = "claude-desktop"
)

var configLibraryIDPattern = regexp.MustCompile(`^[a-f0-9-]{36}$`)

type fileSnapshot struct {
	Path     string `json:"path"`
	Present  bool   `json:"present"`
	Contents []byte `json:"contents,omitempty"`
}

type desktopBackup struct {
	Files []fileSnapshot `json:"files"`
}

type desktopPaths struct {
	StandardConfig   string
	ThirdPartyConfig string
	LibraryMeta      string
	LibraryProfile   string
	Backup           string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Claudex Desktop for Linux: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if runtime.GOOS != "linux" {
		return errors.New("this launcher requires Linux; use ClaudexDesktop.app on macOS")
	}

	configPath := resolveConfigPath()
	cfg, _, err := claudex.LoadServeConfig(configPath)
	if err != nil {
		return err
	}
	localKey, err := claudex.LocalAPIKey(cfg)
	if err != nil {
		return err
	}
	if !hasAuthMaterial(cfg.AuthDir) {
		return fmt.Errorf("Codex authentication is not configured; run claudex-server login --config %s first", configPath)
	}

	paths, err := resolveDesktopPaths()
	if err != nil {
		return err
	}
	if err = restorePendingBackup(paths.Backup); err != nil {
		return err
	}
	if err = stopRunningDesktop(); err != nil {
		return err
	}

	backup, err := captureDesktopFiles(paths)
	if err != nil {
		return err
	}
	if err = writeBackup(paths.Backup, backup); err != nil {
		return err
	}

	var restoreOnce sync.Once
	var restoreErr error
	restore := func() {
		restoreOnce.Do(func() {
			restoreErr = restoreDesktopFiles(backup)
			if restoreErr == nil {
				_ = os.Remove(paths.Backup)
			}
		})
	}
	defer restore()

	if err = applyGatewayConfiguration(paths, claudex.ServerURL(cfg), localKey); err != nil {
		return err
	}

	if !claudex.WaitForServer(cfg, 1) {
		serverPath, errServerPath := resolveServerPath()
		if errServerPath != nil {
			return errServerPath
		}
		if err = startServer(serverPath, configPath); err != nil {
			return err
		}
		if !claudex.WaitForServer(cfg, 60) {
			return fmt.Errorf("Claudex server did not become ready at %s; see ~/.claudex/desktop-linux-serve.stderr.log", claudex.ServerURL(cfg))
		}
	}

	desktopCommand := strings.TrimSpace(os.Getenv("CLAUDEX_CLAUDE_DESKTOP_COMMAND"))
	if desktopCommand == "" {
		desktopCommand = defaultDesktopBinary
	}
	desktopPath, err := exec.LookPath(desktopCommand)
	if err != nil {
		return fmt.Errorf("Claude Desktop command not found: %s", desktopCommand)
	}

	cmd := exec.Command(desktopPath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	signalStop := make(chan os.Signal, 1)
	signal.Notify(signalStop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalStop)
	go func() {
		<-signalStop
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
		restore()
	}()

	fmt.Printf("Starting Claude Desktop with Claudex at %s\n", claudex.ServerURL(cfg))
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("run Claude Desktop: %w", err)
	}
	restore()
	if restoreErr != nil {
		return fmt.Errorf("restore Claude Desktop configuration: %w", restoreErr)
	}
	return nil
}

func resolveConfigPath() string {
	if configured := strings.TrimSpace(os.Getenv("CLAUDEX_CONFIG")); configured != "" {
		return expandHome(configured)
	}
	return filepath.Join(xdgConfigHome(), "claudex", configFileName)
}

func resolveServerPath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("CLAUDEX_SERVER_PATH")); configured != "" {
		path := expandHome(configured)
		if isExecutable(path) {
			return path, nil
		}
		return "", fmt.Errorf("Claudex server binary not found: %s", path)
	}

	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), "claudex-server")
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	candidate := filepath.Join(xdgBinHome(), "claudex-server")
	if isExecutable(candidate) {
		return candidate, nil
	}
	return "", fmt.Errorf("Claudex server binary not found; run just setup or set CLAUDEX_SERVER_PATH")
}

func resolveDesktopPaths() (desktopPaths, error) {
	configHome := xdgConfigHome()
	thirdPartyRoot := filepath.Join(configHome, "Claude-3p")
	metaPath := filepath.Join(thirdPartyRoot, "configLibrary", "_meta.json")
	profileID, err := currentOrNewProfileID(metaPath)
	if err != nil {
		return desktopPaths{}, err
	}
	return desktopPaths{
		StandardConfig:   filepath.Join(configHome, "Claude", "claude_desktop_config.json"),
		ThirdPartyConfig: filepath.Join(thirdPartyRoot, "claude_desktop_config.json"),
		LibraryMeta:      metaPath,
		LibraryProfile:   filepath.Join(filepath.Dir(metaPath), profileID+".json"),
		Backup:           filepath.Join(configHome, "claudex", backupFileName),
	}, nil
}

func currentOrNewProfileID(metaPath string) (string, error) {
	contents, err := os.ReadFile(metaPath)
	if errors.Is(err, os.ErrNotExist) {
		return newUUID()
	}
	if err != nil {
		return "", fmt.Errorf("read Claude Desktop config library metadata: %w", err)
	}
	var meta map[string]any
	if err = json.Unmarshal(contents, &meta); err != nil {
		return "", fmt.Errorf("decode Claude Desktop config library metadata: %w", err)
	}
	appliedID, _ := meta["appliedId"].(string)
	if !configLibraryIDPattern.MatchString(appliedID) {
		return "", errors.New("Claude Desktop config library metadata has an invalid applied configuration")
	}
	return appliedID, nil
}

func captureDesktopFiles(paths desktopPaths) (desktopBackup, error) {
	filePaths := []string{
		paths.StandardConfig,
		paths.ThirdPartyConfig,
		paths.LibraryMeta,
		paths.LibraryProfile,
	}
	backup := desktopBackup{Files: make([]fileSnapshot, 0, len(filePaths))}
	for _, path := range filePaths {
		snapshot, err := captureFile(path)
		if err != nil {
			return desktopBackup{}, err
		}
		backup.Files = append(backup.Files, snapshot)
	}
	return backup, nil
}

func captureFile(path string) (fileSnapshot, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileSnapshot{Path: path}, nil
	}
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("read %s: %w", path, err)
	}
	return fileSnapshot{Path: path, Present: true, Contents: contents}, nil
}

func writeBackup(path string, backup desktopBackup) error {
	contents, err := json.Marshal(backup)
	if err != nil {
		return fmt.Errorf("encode Claude Desktop backup: %w", err)
	}
	if err = writePrivateFile(path, contents); err != nil {
		return fmt.Errorf("write Claude Desktop backup: %w", err)
	}
	return nil
}

func restorePendingBackup(path string) error {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pending Claude Desktop backup: %w", err)
	}
	var backup desktopBackup
	if err = json.Unmarshal(contents, &backup); err != nil {
		return fmt.Errorf("decode pending Claude Desktop backup: %w", err)
	}
	if err = restoreDesktopFiles(backup); err != nil {
		return fmt.Errorf("restore pending Claude Desktop backup: %w", err)
	}
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove pending Claude Desktop backup: %w", err)
	}
	return nil
}

func restoreDesktopFiles(backup desktopBackup) error {
	for index := len(backup.Files) - 1; index >= 0; index-- {
		snapshot := backup.Files[index]
		if snapshot.Present {
			if err := writePrivateFile(snapshot.Path, snapshot.Contents); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(snapshot.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func applyGatewayConfiguration(paths desktopPaths, baseURL, apiKey string) error {
	for _, configPath := range []string{paths.StandardConfig, paths.ThirdPartyConfig} {
		config, err := readJSONObject(configPath)
		if err != nil {
			return err
		}
		config["deploymentMode"] = "3p"
		if err = writeJSONObject(configPath, config); err != nil {
			return err
		}
	}

	profile, err := readJSONObject(paths.LibraryProfile)
	if err != nil {
		return err
	}
	models, err := inferenceModels()
	if err != nil {
		return err
	}
	profile["disableDeploymentModeChooser"] = true
	profile["inferenceProvider"] = "gateway"
	profile["inferenceGatewayBaseUrl"] = strings.TrimRight(baseURL, "/")
	profile["inferenceGatewayApiKey"] = apiKey
	profile["inferenceGatewayAuthScheme"] = "bearer"
	profile["inferenceModels"] = models
	delete(profile, "inferenceCredentialKind")
	delete(profile, "inferenceGatewayOidc")
	if err = writeJSONObject(paths.LibraryProfile, profile); err != nil {
		return err
	}

	profileID := strings.TrimSuffix(filepath.Base(paths.LibraryProfile), filepath.Ext(paths.LibraryProfile))
	meta, err := readJSONObject(paths.LibraryMeta)
	if err != nil {
		return err
	}
	meta["appliedId"] = profileID
	meta["entries"] = []map[string]string{{
		"id":       profileID,
		"name":     "Claudex",
		"provider": "gateway",
	}}
	return writeJSONObject(paths.LibraryMeta, meta)
}

func inferenceModels() (any, error) {
	var models any
	if err := json.Unmarshal([]byte(claudex.InferenceModelsValue()), &models); err != nil {
		return nil, fmt.Errorf("decode Claudex model profiles: %w", err)
	}
	return models, nil
}

func readJSONObject(path string) (map[string]any, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var object map[string]any
	if err = json.Unmarshal(contents, &object); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if object == nil {
		object = make(map[string]any)
	}
	return object, nil
}

func writeJSONObject(path string, object map[string]any) error {
	contents, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	contents = append(contents, '\n')
	if err = writePrivateFile(path, contents); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writePrivateFile(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".claudex-desktop-*")
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

func startServer(serverPath, configPath string) error {
	logDir := filepath.Join(expandHome("~"), ".claudex")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return fmt.Errorf("create Claudex log directory: %w", err)
	}
	stdout, err := os.OpenFile(filepath.Join(logDir, "desktop-linux-serve.stdout.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	stderr, err := os.OpenFile(filepath.Join(logDir, "desktop-linux-serve.stderr.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		_ = stdout.Close()
		return err
	}
	cmd := exec.Command(serverPath, "serve", "--config", configPath)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err = cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return fmt.Errorf("start Claudex server: %w", err)
	}
	_ = stdout.Close()
	_ = stderr.Close()
	return nil
}

func stopRunningDesktop() error {
	pid := runningDesktopPID()
	if pid <= 0 {
		return nil
	}
	fmt.Printf("Stopping running Claude Desktop process %d so it reloads the Claudex profile\n", pid)
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("stop Claude Desktop process %d: %w", pid, err)
	}
	for attempt := 0; attempt < 120; attempt++ {
		if !processAlive(pid) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("Claude Desktop process %d did not exit", pid)
}

func runningDesktopPID() int {
	stateHome := strings.TrimSpace(os.Getenv("XDG_STATE_HOME"))
	if stateHome == "" {
		stateHome = filepath.Join(expandHome("~"), ".local", "state")
	}
	pidPath := filepath.Join(stateHome, "claude-desktop", "app.pid")
	if contents, err := os.ReadFile(pidPath); err == nil {
		if pid, errParse := strconv.Atoi(strings.TrimSpace(string(contents))); errParse == nil && processAlive(pid) {
			return pid
		}
	}
	output, err := exec.Command("pgrep", "-x", "claude-desktop").Output()
	if err != nil {
		return 0
	}
	for _, line := range strings.Fields(string(output)) {
		pid, errParse := strconv.Atoi(line)
		if errParse == nil && processAlive(pid) {
			return pid
		}
	}
	return 0
}

func processAlive(pid int) bool {
	return pid > 0 && syscall.Kill(pid, 0) == nil
}

func hasAuthMaterial(path string) bool {
	entries, err := os.ReadDir(expandHome(path))
	return err == nil && len(entries) > 0
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func xdgConfigHome() string {
	if configured := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); configured != "" {
		return expandHome(configured)
	}
	return filepath.Join(expandHome("~"), ".config")
}

func xdgBinHome() string {
	if configured := strings.TrimSpace(os.Getenv("XDG_BIN_HOME")); configured != "" {
		return expandHome(configured)
	}
	return filepath.Join(expandHome("~"), ".local", "bin")
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

func newUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}
