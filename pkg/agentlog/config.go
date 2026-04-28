package agentlog

import (
	"os"
	"path/filepath"
	"strings"
)

type BackendConfig struct {
	Type string
	Args map[string]string
}

type Config struct {
	Version  int
	UserID   string
	Backends []BackendConfig
}

func DefaultConfig() Config {
	return Config{
		Version: 1,
		UserID:  defaultString(os.Getenv("USER"), "unknown"),
		Backends: []BackendConfig{
			{
				Type: "markdown",
				Args: map[string]string{"path": ".agentlog/sessions"},
			},
		},
	}
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func ConfigPath(root string) string {
	return filepath.Join(root, ".agentlog", "config.yaml")
}

func LoadConfig(root string) (Config, error) {
	cfg := DefaultConfig()
	path := ConfigPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	lines := strings.Split(string(data), "\n")
	var current BackendConfig
	inBackends := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "user_id:") {
			cfg.UserID = strings.TrimSpace(strings.TrimPrefix(line, "user_id:"))
			continue
		}
		if strings.HasPrefix(line, "backends:") {
			inBackends = true
			continue
		}
		if !inBackends {
			continue
		}
		if strings.HasPrefix(line, "- type:") {
			if current.Type != "" {
				cfg.Backends = append(cfg.Backends, current)
			}
			current = BackendConfig{
				Type: strings.TrimSpace(strings.TrimPrefix(line, "- type:")),
				Args: map[string]string{},
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && current.Type != "" {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			current.Args[k] = v
		}
	}
	if current.Type != "" {
		cfg.Backends = append(cfg.Backends, current)
	}
	if len(cfg.Backends) == 0 {
		cfg.Backends = DefaultConfig().Backends
	}
	return cfg, nil
}

func WriteDefaultConfig(root string) error {
	path := ConfigPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	content := "version: 1\nuser_id: " + defaultString(os.Getenv("USER"), "unknown") + "\nbackends:\n  - type: markdown\n    path: .agentlog/sessions\n"
	return os.WriteFile(path, []byte(content), 0o644)
}
