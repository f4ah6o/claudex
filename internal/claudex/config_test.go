package claudex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfigBytesRejectsUnknownFields(t *testing.T) {
	_, err := ParseConfigBytes([]byte("host: 127.0.0.1\nunknown-setting: true\n"))
	if err == nil || !strings.Contains(err.Error(), "unknown-setting") {
		t.Fatalf("ParseConfigBytes() error = %v, want unknown field", err)
	}
}

func TestParseConfigBytesRejectsDuplicateFields(t *testing.T) {
	_, err := ParseConfigBytes([]byte("host: 127.0.0.1\nport: 8317\nport: 8318\n"))
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("ParseConfigBytes() error = %v, want duplicate field", err)
	}
}

func TestParseConfigBytesRejectsUnsafeAliases(t *testing.T) {
	data := []byte("" +
		"host: 127.0.0.1\n" +
		"port: 8317\n" +
		"oauth-model-alias:\n" +
		"  codex:\n" +
		"    - name: gpt-4o\n" +
		"      alias: claude-opus-5\n")
	_, err := ParseConfigBytes(data)
	if err == nil || !strings.Contains(err.Error(), "gpt-5.6") {
		t.Fatalf("ParseConfigBytes() error = %v, want model family rejection", err)
	}
}

func TestParseConfigBytesLoadsExample(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "claudex.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseConfigBytes(data)
	if err != nil {
		t.Fatalf("ParseConfigBytes(example) error = %v", err)
	}
	if cfg.Host != DefaultHost || cfg.Port != DefaultPort || len(cfg.APIKeys) != 1 {
		t.Fatalf("parsed example config = %#v", cfg)
	}
}

func TestEnsureConfigGeneratesPrivateRandomKey(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.yaml")
	configPath := filepath.Join(dir, "nested", "claudex.yaml")
	if err := os.WriteFile(templatePath, []byte("api-keys:\n  - replace-with-a-local-random-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	created, err := EnsureConfig(configPath, templatePath)
	if err != nil || !created {
		t.Fatalf("EnsureConfig() = %t, %v", created, err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "replace-with-a-local-random-key") {
		t.Fatal("generated config still contains placeholder key")
	}
}

func TestLoadConfigRejectsInsecureFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claudex.yaml")
	if err := os.WriteFile(path, []byte("host: 127.0.0.1\nport: 8317\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadConfig(path); err == nil || !strings.Contains(err.Error(), "owner-only") {
		t.Fatalf("LoadConfig() error = %v, want owner-only mode error", err)
	}
}

func TestEnsureConfigRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.yaml")
	path := filepath.Join(dir, "claudex.yaml")
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := EnsureConfig(path, target); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("EnsureConfig() error = %v, want symlink rejection", err)
	}
}
