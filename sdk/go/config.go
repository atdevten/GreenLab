package sdk

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const defaultConfigFile = "~/.greenlab/sdk.json"

// localConfig holds persisted SDK state across reboots.
type localConfig struct {
	ChannelID     string `json:"channel_id"`
	SchemaVersion uint32 `json:"schema_version"`
	Format        string `json:"format"`
}

// expandPath expands a leading ~ to the user's home directory.
func expandPath(p string) (string, error) {
	if len(p) == 0 {
		return p, nil
	}
	if p[0] != '~' {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, p[1:]), nil
}

// loadConfig reads the local config file. If the file does not exist, it
// returns an empty config without error.
func loadConfig(path string) (localConfig, error) {
	expanded, err := expandPath(path)
	if err != nil {
		return localConfig{}, err
	}

	data, err := os.ReadFile(expanded)
	if os.IsNotExist(err) {
		return localConfig{}, nil
	}
	if err != nil {
		return localConfig{}, err
	}

	var cfg localConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return localConfig{}, err
	}
	return cfg, nil
}

// saveConfig writes the local config file, creating parent directories as needed.
func saveConfig(path string, cfg localConfig) error {
	expanded, err := expandPath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(expanded), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(expanded, data, 0o600)
}
