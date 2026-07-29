package claudex

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
	"gopkg.in/yaml.v3"
)

// PublicConfig is the deliberately small configuration contract exposed by
// Claudex. The generic upstream configuration remains an internal adapter.
type PublicConfig struct {
	Host                   string                             `yaml:"host"`
	Port                   int                                `yaml:"port"`
	AuthDir                string                             `yaml:"auth-dir"`
	APIKeys                []string                           `yaml:"api-keys"`
	Debug                  bool                               `yaml:"debug"`
	LoggingToFile          bool                               `yaml:"logging-to-file"`
	UsageStatisticsEnabled bool                               `yaml:"usage-statistics-enabled"`
	DisableImageGeneration bool                               `yaml:"disable-image-generation"`
	RequestRetry           int                                `yaml:"request-retry"`
	MaxRetryInterval       int                                `yaml:"max-retry-interval"`
	OAuthModelAlias        map[string][]PublicOAuthModelAlias `yaml:"oauth-model-alias"`
}

// PublicOAuthModelAlias is the supported client-visible model mapping.
type PublicOAuthModelAlias struct {
	Name         string `yaml:"name"`
	Alias        string `yaml:"alias"`
	Fork         bool   `yaml:"fork"`
	ForceMapping bool   `yaml:"force-mapping"`
}

// ParseConfigBytes strictly decodes the Claudex-owned YAML schema and maps it
// to the internal configuration consumed by the shared runtime.
func ParseConfigBytes(data []byte) (*config.Config, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("Claudex configuration is empty")
	}
	if err := rejectDuplicateYAMLKeys(data); err != nil {
		return nil, err
	}

	var public PublicConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&public); err != nil {
		return nil, fmt.Errorf("invalid Claudex configuration: %w; remove unsupported legacy fields or migrate them to the Claudex schema", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("invalid Claudex configuration: multiple YAML documents are not supported")
		}
		return nil, fmt.Errorf("invalid Claudex configuration: %w", err)
	}

	cfg := &config.Config{
		Host:    public.Host,
		Port:    public.Port,
		AuthDir: public.AuthDir,
		SDKConfig: internalconfig.SDKConfig{
			APIKeys:                append([]string(nil), public.APIKeys...),
			DisableImageGeneration: internalconfig.DisableImageGenerationOff,
		},
		Debug:                  public.Debug,
		LoggingToFile:          public.LoggingToFile,
		UsageStatisticsEnabled: public.UsageStatisticsEnabled,
		RequestRetry:           public.RequestRetry,
		MaxRetryInterval:       public.MaxRetryInterval,
		WebsocketAuth:          true,
	}
	if public.DisableImageGeneration {
		cfg.DisableImageGeneration = internalconfig.DisableImageGenerationAll
	}
	cfg.OAuthModelAlias = make(map[string][]config.OAuthModelAlias, len(public.OAuthModelAlias))
	for provider, aliases := range public.OAuthModelAlias {
		mapped := make([]config.OAuthModelAlias, 0, len(aliases))
		for _, alias := range aliases {
			mapped = append(mapped, config.OAuthModelAlias{
				Name:         alias.Name,
				Alias:        alias.Alias,
				Fork:         alias.Fork,
				ForceMapping: alias.ForceMapping,
			})
		}
		cfg.OAuthModelAlias[provider] = mapped
	}
	if err := validatePublicConfig(cfg); err != nil {
		return nil, err
	}
	Normalize(cfg)
	return cfg, nil
}

func validatePublicConfig(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("Claudex configuration is required")
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", cfg.Port)
	}
	if cfg.RequestRetry < 0 || cfg.RequestRetry > 10 {
		return fmt.Errorf("request-retry must be between 0 and 10, got %d", cfg.RequestRetry)
	}
	if cfg.MaxRetryInterval < 0 || cfg.MaxRetryInterval > 3600 {
		return fmt.Errorf("max-retry-interval must be between 0 and 3600 seconds, got %d", cfg.MaxRetryInterval)
	}

	seen := make(map[string]string)
	for provider, aliases := range cfg.OAuthModelAlias {
		if !strings.EqualFold(strings.TrimSpace(provider), "codex") && len(aliases) > 0 {
			return fmt.Errorf("oauth-model-alias.%s is unsupported; Claudex accepts only Codex aliases", provider)
		}
		for index, alias := range aliases {
			name := strings.TrimSpace(alias.Name)
			clientID := strings.TrimSpace(alias.Alias)
			if clientID == "" || name == "" {
				return fmt.Errorf("oauth-model-alias.%s[%d] requires non-empty name and alias", provider, index)
			}
			if isHaikuModelAlias(clientID) {
				return fmt.Errorf("oauth-model-alias.%s[%d].alias %q is unsupported", provider, index, clientID)
			}
			if !IsGPT56Model(name) {
				return fmt.Errorf("oauth-model-alias.%s[%d].name %q is outside the supported gpt-5.6 family", provider, index, name)
			}
			key := strings.ToLower(clientID)
			if previous, exists := seen[key]; exists {
				return fmt.Errorf("duplicate model alias %q in %s and %s", clientID, previous, provider)
			}
			seen[key] = provider
		}
	}
	return nil
}

func rejectDuplicateYAMLKeys(data []byte) error {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("invalid Claudex YAML: %w", err)
	}
	var walk func(*yaml.Node, string) error
	walk = func(node *yaml.Node, path string) error {
		if node == nil {
			return nil
		}
		switch node.Kind {
		case yaml.DocumentNode:
			for _, child := range node.Content {
				if err := walk(child, path); err != nil {
					return err
				}
			}
		case yaml.MappingNode:
			seen := make(map[string]struct{}, len(node.Content)/2)
			for index := 0; index+1 < len(node.Content); index += 2 {
				keyNode := node.Content[index]
				valueNode := node.Content[index+1]
				key := keyNode.Value
				if _, exists := seen[key]; exists {
					fieldPath := key
					if path != "" {
						fieldPath = path + "." + key
					}
					return fmt.Errorf("duplicate configuration field %q", fieldPath)
				}
				seen[key] = struct{}{}
				fieldPath := key
				if path != "" {
					fieldPath = path + "." + key
				}
				if err := walk(valueNode, fieldPath); err != nil {
					return err
				}
			}
		case yaml.SequenceNode:
			for index, child := range node.Content {
				if err := walk(child, fmt.Sprintf("%s[%d]", path, index)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(&document, "")
}

// EnsureConfig creates a private initial configuration from a template.
func EnsureConfig(path, templatePath string) (bool, error) {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("configuration path must not be a symlink: %s", path)
		}
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect configuration path: %w", err)
	}
	contents, err := os.ReadFile(templatePath)
	if err != nil {
		return false, fmt.Errorf("read configuration template: %w", err)
	}
	keyBytes := make([]byte, 32)
	if _, err = rand.Read(keyBytes); err != nil {
		return false, fmt.Errorf("generate local API key: %w", err)
	}
	key := hex.EncodeToString(keyBytes)
	contents = []byte(strings.ReplaceAll(string(contents), "replace-with-a-local-random-key", key))
	if err = writePrivateConfigFile(path, contents); err != nil {
		return false, fmt.Errorf("create configuration: %w", err)
	}
	return true, nil
}

func writePrivateConfigFile(path string, contents []byte) error {
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
	return os.Rename(temporaryName, path)
}

func validateConfigFileMode(info os.FileInfo, path string) error {
	if runtime.GOOS == "windows" || !info.Mode().IsRegular() {
		return nil
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("configuration file must be owner-only (mode 0600 or stricter): %s has mode %o", path, info.Mode().Perm())
	}
	return nil
}
