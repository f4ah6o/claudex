package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/claudex"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
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
	Present  bool   `json:"present"`
	Value    string `json:"value,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

type preferenceSnapshot struct {
	Values               map[string]preferenceValue `json:"values"`
	ConfigLibrary        configLibrarySnapshot      `json:"configLibrary"`
	AppliedValues        map[string]preferenceValue `json:"appliedValues,omitempty"`
	AppliedConfigLibrary configLibrarySnapshot      `json:"appliedConfigLibrary,omitempty"`
}

type fileSnapshot struct {
	Present  bool   `json:"present"`
	Contents []byte `json:"contents,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

type desktopSessionLock struct {
	PID          int       `json:"pid"`
	StartedAt    time.Time `json:"startedAt"`
	Executable   string    `json:"executable"`
	ProcessStart string    `json:"processStart"`
	Transaction  string    `json:"transactionId"`
}

type desktopSessionOwner struct {
	path string
	lock desktopSessionLock
}

type configLibrarySnapshot struct {
	MetaPath   string       `json:"metaPath"`
	ConfigPath string       `json:"configPath"`
	Meta       fileSnapshot `json:"meta"`
	Config     fileSnapshot `json:"config"`
}

type configLibraryMeta struct {
	AppliedID string               `json:"appliedId"`
	Entries   []configLibraryEntry `json:"entries"`
}

type configLibraryEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var configLibraryIDPattern = regexp.MustCompile(`^[a-f0-9-]{36}$`)

func main() {
	if err := run(); err != nil {
		_ = showMessage("ClaudexDesktop", err.Error(), false)
		fmt.Fprintf(os.Stderr, "ClaudexDesktop: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if runtime.GOOS == "linux" {
		return runLinux()
	}
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("ClaudexDesktop requires macOS or Linux")
	}
	return runDarwin()
}

func runDarwin() error {

	configPath := resolveConfigPath()
	if absolutePath, errAbs := filepath.Abs(configPath); errAbs == nil {
		configPath = absolutePath
	}
	owner, err := acquireDesktopSession(preferenceLockPath(configPath))
	if err != nil {
		return err
	}
	defer owner.release()
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

	cfg, _, err := claudex.LoadServeConfig(configPath)
	if err != nil {
		return err
	}
	localKey, err := claudex.LocalAPIKey(cfg)
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
	appliedStateCaptured := false
	var restoreOnce sync.Once
	restore := func() {
		restoreOnce.Do(func() {
			var errRestore error
			if appliedStateCaptured {
				errRestore = snapshot.restore()
			} else {
				errRestore = snapshot.restoreOriginal()
			}
			if errRestore != nil {
				logMessage("could not restore Claude Desktop preferences: " + errRestore.Error())
				return
			}
			_ = os.Remove(pendingPath)
		})
	}
	defer restore()
	if err = applyGatewayPreferences(claudex.ServerURL(cfg), localKey, snapshot.ConfigLibrary); err != nil {
		return err
	}
	applied, err := capturePreferences()
	if err != nil {
		return fmt.Errorf("capture Claudex preference transaction: %w", err)
	}
	snapshot.AppliedValues = applied.Values
	snapshot.AppliedConfigLibrary = applied.ConfigLibrary
	appliedStateCaptured = true
	if err = writePreferenceBackup(pendingPath, snapshot); err != nil {
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
	created, err := claudex.EnsureConfig(path, templatePath)
	if err != nil {
		return false, err
	}
	return created, nil
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
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporaryName, path); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() {
		_ = directory.Close()
	}()
	return directory.Sync()
}

func hasAuthMaterial(path string) bool {
	path = expandHome(path)
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func startServer(cfg *config.Config, configPath string) (*exec.Cmd, error) {
	if claudex.WaitForServer(cfg, 1) {
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
	if !claudex.WaitForServer(cfg, 60) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("Claudex server did not become ready at %s; see ~/.claudex/desktop-serve.stderr.log", claudex.ServerURL(cfg))
	}
	return cmd, nil
}

func capturePreferences() (preferenceSnapshot, error) {
	snapshot := preferenceSnapshot{Values: make(map[string]preferenceValue, len(desktopPreferenceKeys))}
	for _, key := range desktopPreferenceKeys {
		value, present, err := readPreference(key)
		if err != nil {
			return preferenceSnapshot{}, err
		}
		snapshot.Values[key] = newPreferenceValue(present, value)
	}
	configLibrary, err := captureConfigLibrary()
	if err != nil {
		return preferenceSnapshot{}, err
	}
	snapshot.ConfigLibrary = configLibrary
	return snapshot, nil
}

func (snapshot preferenceSnapshot) restore() error {
	if err := snapshot.verifyAppliedState(); err != nil {
		return err
	}
	return snapshot.restoreOriginal()
}

func (snapshot preferenceSnapshot) restoreOriginal() error {
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
	if err := restoreConfigLibrary(snapshot.ConfigLibrary); err != nil {
		return err
	}
	return nil
}

func (snapshot preferenceSnapshot) verifyAppliedState() error {
	if len(snapshot.AppliedValues) != len(desktopPreferenceKeys) || snapshot.AppliedConfigLibrary.MetaPath == "" {
		return errors.New("Claudex Desktop transaction has no owned-state checksum; restore the saved preferences manually after reviewing desktop-preferences-backup.json")
	}
	for _, key := range desktopPreferenceKeys {
		want, ok := snapshot.AppliedValues[key]
		if !ok {
			return fmt.Errorf("Claudex Desktop transaction is incomplete for preference %s; manual recovery is required", key)
		}
		value, present, err := readPreference(key)
		if err != nil {
			return fmt.Errorf("verify current preference %s: %w", key, err)
		}
		if got := newPreferenceValue(present, value); got != want {
			return fmt.Errorf("Claude Desktop preference %s changed outside Claudex; leaving it untouched for manual recovery", key)
		}
	}
	for path, want := range map[string]fileSnapshot{
		snapshot.AppliedConfigLibrary.ConfigPath: snapshot.AppliedConfigLibrary.Config,
		snapshot.AppliedConfigLibrary.MetaPath:   snapshot.AppliedConfigLibrary.Meta,
	} {
		got, err := readFileSnapshot(path)
		if err != nil {
			return fmt.Errorf("verify current Claude Desktop file %s: %w", path, err)
		}
		if got.Present != want.Present || got.Checksum != want.Checksum {
			return fmt.Errorf("Claude Desktop file %s changed outside Claudex; leaving it untouched for manual recovery", path)
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

func preferenceLockPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "desktop-session.lock")
}

func acquireDesktopSession(path string) (*desktopSessionOwner, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("identify ClaudexDesktop executable: %w", err)
	}
	lock := desktopSessionLock{
		PID:          os.Getpid(),
		StartedAt:    time.Now().UTC(),
		Executable:   executable,
		ProcessStart: processStartIdentity(os.Getpid()),
		Transaction:  newTransactionID(),
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create Claudex Desktop session directory: %w", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr == nil {
			contents, marshalErr := json.Marshal(lock)
			if marshalErr == nil {
				_, marshalErr = file.Write(contents)
			}
			if marshalErr == nil {
				marshalErr = file.Sync()
			}
			closeErr := file.Close()
			if marshalErr != nil {
				_ = os.Remove(path)
				return nil, fmt.Errorf("write Claudex Desktop session lock: %w", marshalErr)
			}
			if closeErr != nil {
				_ = os.Remove(path)
				return nil, fmt.Errorf("close Claudex Desktop session lock: %w", closeErr)
			}
			return &desktopSessionOwner{path: path, lock: lock}, nil
		}
		if !errors.Is(createErr, os.ErrExist) {
			return nil, fmt.Errorf("create Claudex Desktop session lock: %w", createErr)
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("read existing Claudex Desktop session lock: %w", readErr)
		}
		var existing desktopSessionLock
		if unmarshalErr := json.Unmarshal(contents, &existing); unmarshalErr != nil {
			return nil, errors.New("existing Claudex Desktop session lock is invalid; remove it only after confirming no launcher is running")
		}
		if sessionOwnerAlive(existing) {
			return nil, fmt.Errorf("another ClaudexDesktop session owns preferences (pid %d, transaction %s)", existing.PID, existing.Transaction)
		}
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return nil, fmt.Errorf("remove stale Claudex Desktop session lock: %w", removeErr)
		}
	}
	return nil, errors.New("could not acquire Claudex Desktop session lock")
}

func (owner *desktopSessionOwner) release() {
	if owner == nil {
		return
	}
	contents, err := os.ReadFile(owner.path)
	if err != nil {
		return
	}
	var current desktopSessionLock
	if json.Unmarshal(contents, &current) == nil && current.Transaction == owner.lock.Transaction {
		_ = os.Remove(owner.path)
	}
}

func sessionOwnerAlive(lock desktopSessionLock) bool {
	if lock.PID <= 0 {
		return false
	}
	process, err := os.FindProcess(lock.PID)
	if err != nil || process.Signal(syscall.Signal(0)) != nil {
		return false
	}
	if lock.Executable != "" {
		identity := executableIdentity(lock.PID)
		if identity != lock.Executable && filepath.Base(identity) != filepath.Base(lock.Executable) {
			return false
		}
	}
	return lock.ProcessStart != "" && processStartIdentity(lock.PID) == lock.ProcessStart
}

func executableIdentity(pid int) string {
	if pid == os.Getpid() {
		path, _ := os.Executable()
		return path
	}
	if runtime.GOOS == "linux" {
		path, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		return path
	}
	output, err := exec.Command("/bin/ps", "-p", fmt.Sprint(pid), "-o", "comm=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func processStartIdentity(pid int) string {
	if runtime.GOOS == "linux" {
		contents, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			return ""
		}
		line := string(contents)
		if closing := strings.LastIndex(line, ")"); closing >= 0 {
			fields := strings.Fields(line[closing+1:])
			if len(fields) > 19 {
				return fields[19]
			}
		}
		return ""
	}
	output, err := exec.Command("/bin/ps", "-p", fmt.Sprint(pid), "-o", "lstart=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func newTransactionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("pid-%d-%d", os.Getpid(), time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func applyGatewayPreferences(baseURL, apiKey string, library configLibrarySnapshot) error {
	if err := writeGatewayConfigLibrary(baseURL, apiKey, library); err != nil {
		return err
	}
	if err := writePreference("inferenceGatewayBaseUrl", baseURL); err != nil {
		return err
	}
	if err := writePreference("inferenceGatewayApiKey", apiKey); err != nil {
		return err
	}
	if err := writePreference("inferenceGatewayAuthScheme", "bearer"); err != nil {
		return err
	}
	for _, key := range []string{"inferenceCredentialKind", "inferenceGatewayOidc"} {
		if err := deletePreference(key); err != nil {
			return err
		}
	}
	if err := writePreference("inferenceModels", claudex.InferenceModelsValue()); err != nil {
		return err
	}
	return writePreference("inferenceProvider", "gateway")
}

func captureConfigLibrary() (configLibrarySnapshot, error) {
	metaPath := filepath.Join(claudeConfigLibraryDir(), "_meta.json")
	meta, err := readFileSnapshot(metaPath)
	if err != nil {
		return configLibrarySnapshot{}, fmt.Errorf("read Claude Desktop config library metadata: %w", err)
	}
	configPath, err := configLibraryConfigPath(metaPath, meta)
	if err != nil {
		return configLibrarySnapshot{}, err
	}
	config, err := readFileSnapshot(configPath)
	if err != nil {
		return configLibrarySnapshot{}, fmt.Errorf("read Claude Desktop config library: %w", err)
	}
	return configLibrarySnapshot{
		MetaPath:   metaPath,
		ConfigPath: configPath,
		Meta:       meta,
		Config:     config,
	}, nil
}

func writeGatewayConfigLibrary(baseURL, apiKey string, library configLibrarySnapshot) error {
	metaPath := library.MetaPath
	if metaPath == "" {
		metaPath = filepath.Join(claudeConfigLibraryDir(), "_meta.json")
	}
	configPath := library.ConfigPath
	if configPath == "" {
		var err error
		configPath, err = configLibraryConfigPath(metaPath, library.Meta)
		if err != nil {
			return err
		}
	}

	config := make(map[string]json.RawMessage)
	if existing, errRead := os.ReadFile(configPath); errRead == nil {
		if errUnmarshal := json.Unmarshal(existing, &config); errUnmarshal != nil {
			return fmt.Errorf("decode Claude Desktop config library: %w", errUnmarshal)
		}
	} else if !errors.Is(errRead, os.ErrNotExist) {
		return fmt.Errorf("read Claude Desktop config library: %w", errRead)
	}
	if config == nil {
		config = make(map[string]json.RawMessage)
	}
	setConfigString(config, "inferenceProvider", "gateway")
	setConfigString(config, "inferenceGatewayBaseUrl", baseURL)
	setConfigString(config, "inferenceGatewayApiKey", apiKey)
	setConfigString(config, "inferenceGatewayAuthScheme", "bearer")
	setConfigString(config, "inferenceModels", claudex.InferenceModelsValue())
	delete(config, "inferenceCredentialKind")
	delete(config, "inferenceGatewayOidc")

	contents, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Claude Desktop config library: %w", err)
	}
	if err = writePrivateFile(configPath, contents); err != nil {
		return fmt.Errorf("write Claude Desktop config library: %w", err)
	}
	if library.Meta.Present {
		return nil
	}

	metaID := strings.TrimSuffix(filepath.Base(configPath), filepath.Ext(configPath))
	metaContents, err := json.MarshalIndent(configLibraryMeta{
		AppliedID: metaID,
		Entries:   []configLibraryEntry{{ID: metaID, Name: "Default"}},
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Claude Desktop config library metadata: %w", err)
	}
	if err = writePrivateFile(metaPath, metaContents); err != nil {
		return fmt.Errorf("write Claude Desktop config library metadata: %w", err)
	}
	return nil
}

func restoreConfigLibrary(snapshot configLibrarySnapshot) error {
	if snapshot.MetaPath == "" {
		return nil
	}
	if err := restoreFileSnapshot(snapshot.ConfigPath, snapshot.Config); err != nil {
		return fmt.Errorf("restore Claude Desktop config library: %w", err)
	}
	if err := restoreFileSnapshot(snapshot.MetaPath, snapshot.Meta); err != nil {
		return fmt.Errorf("restore Claude Desktop config library metadata: %w", err)
	}
	return nil
}

func restoreFileSnapshot(path string, snapshot fileSnapshot) error {
	if path == "" {
		return nil
	}
	if snapshot.Present {
		return writePrivateFile(path, snapshot.Contents)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func readFileSnapshot(path string) (fileSnapshot, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileSnapshot{}, nil
	}
	if err != nil {
		return fileSnapshot{}, err
	}
	return fileSnapshot{Present: true, Contents: contents, Checksum: checksumBytes(contents)}, nil
}

func newPreferenceValue(present bool, value string) preferenceValue {
	return preferenceValue{Present: present, Value: value, Checksum: checksumBytes([]byte(value))}
}

func checksumBytes(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

func configLibraryConfigPath(metaPath string, metaSnapshot fileSnapshot) (string, error) {
	if metaSnapshot.Present {
		var meta configLibraryMeta
		if err := json.Unmarshal(metaSnapshot.Contents, &meta); err != nil {
			return "", fmt.Errorf("decode Claude Desktop config library metadata: %w", err)
		}
		if !configLibraryIDPattern.MatchString(meta.AppliedID) {
			return "", errors.New("Claude Desktop config library metadata has an invalid applied configuration")
		}
		return filepath.Join(filepath.Dir(metaPath), meta.AppliedID+".json"), nil
	}
	id, err := newUUID()
	if err != nil {
		return "", fmt.Errorf("generate Claude Desktop config library ID: %w", err)
	}
	return filepath.Join(filepath.Dir(metaPath), id+".json"), nil
}

func claudeConfigLibraryDir() string {
	return filepath.Join(expandHome("~"), "Library", "Application Support", "Claude-3p", "configLibrary")
}

func setConfigString(config map[string]json.RawMessage, key, value string) {
	encoded, _ := json.Marshal(value)
	config[key] = encoded
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
	if runtime.GOOS == "linux" {
		return showMessageLinux(title, message, informational)
	}
	return showMessageDarwin(title, message, informational)
}

func showMessageDarwin(title, message string, informational bool) error {
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
