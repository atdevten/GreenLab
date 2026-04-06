package sdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_FileNotExist(t *testing.T) {
	cfg, err := loadConfig(filepath.Join(t.TempDir(), "nonexistent.json"))
	require.NoError(t, err)
	assert.Equal(t, localConfig{}, cfg)
}

func TestLoadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sdk.json")
	data := localConfig{ChannelID: "ch-1", SchemaVersion: 3, Format: "msgpack"}
	raw, _ := json.Marshal(data)
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	cfg, err := loadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, data, cfg)
}

func TestLoadConfig_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sdk.json")
	require.NoError(t, os.WriteFile(path, []byte("not-json"), 0o600))

	_, err := loadConfig(path)
	require.Error(t, err)
}

func TestSaveConfig_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "sdk.json")
	cfg := localConfig{ChannelID: "ch-1", SchemaVersion: 1, Format: "msgpack"}

	require.NoError(t, saveConfig(path, cfg))

	loaded, err := loadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, cfg, loaded)
}

func TestSaveConfig_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sdk.json")

	first := localConfig{ChannelID: "ch-1", SchemaVersion: 1, Format: "msgpack"}
	require.NoError(t, saveConfig(path, first))

	second := localConfig{ChannelID: "ch-2", SchemaVersion: 5, Format: "ojson"}
	require.NoError(t, saveConfig(path, second))

	loaded, err := loadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, second, loaded)
}

func TestExpandPath_TildeExpansion(t *testing.T) {
	expanded, err := expandPath("~/.greenlab/sdk.json")
	require.NoError(t, err)
	assert.NotContains(t, expanded, "~")
	assert.Contains(t, expanded, ".greenlab")
}

func TestExpandPath_NoTilde(t *testing.T) {
	path := "/absolute/path/sdk.json"
	expanded, err := expandPath(path)
	require.NoError(t, err)
	assert.Equal(t, path, expanded)
}

func TestExpandPath_EmptyPath(t *testing.T) {
	expanded, err := expandPath("")
	require.NoError(t, err)
	assert.Equal(t, "", expanded)
}
