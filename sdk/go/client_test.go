package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schemaHandler returns a test HTTP handler that serves a fixed schema response.
func schemaHandler(fields []fieldEntry, schemaVersion uint32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/channels/test-chan/schema" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schemaEnvelope{
			Success: true,
			Data: schemaData{
				Fields:        fields,
				SchemaVersion: schemaVersion,
			},
		})
	}
}

// ingestHandler returns a handler that records received requests and returns
// the given status code.
func ingestHandler(status int, calls *int32, recommendFmt string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/channels/test-chan/data" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(calls, 1)
		if recommendFmt != "" {
			w.Header().Set("X-Recommended-Format", recommendFmt)
		}
		w.WriteHeader(status)
	}
}

func setupTestServer(t *testing.T, fields []fieldEntry, schemaVersion uint32, ingestStatus int, ingestCalls *int32) (*httptest.Server, Config) {
	t.Helper()
	var sv atomic.Uint32
	sv.Store(schemaVersion)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/channels/test-chan/schema", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schemaEnvelope{
			Success: true,
			Data:    schemaData{Fields: fields, SchemaVersion: sv.Load()},
		})
	})
	mux.HandleFunc("/v1/channels/test-chan/data", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(ingestCalls, 1)
		w.WriteHeader(ingestStatus)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	tmpDir := t.TempDir()
	cfg := Config{
		BaseURL:    srv.URL,
		APIKey:     "test-key",
		ChannelID:  "test-chan",
		BatchSize:  10,
		ConfigFile: filepath.Join(tmpDir, ".greenlab", "sdk.json"),
	}
	return srv, cfg
}

func TestNew_Success(t *testing.T) {
	var calls int32
	fields := []fieldEntry{{Index: 1, Name: "temperature", Type: "float"}}
	_, cfg := setupTestServer(t, fields, 1, http.StatusCreated, &calls)

	client, err := New(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, uint32(1), client.schema.schemaVersion)
}

func TestNew_MissingBaseURL(t *testing.T) {
	_, err := New(Config{APIKey: "k", ChannelID: "c"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BaseURL")
}

func TestNew_MissingAPIKey(t *testing.T) {
	_, err := New(Config{BaseURL: "http://x", ChannelID: "c"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APIKey")
}

func TestNew_MissingChannelID(t *testing.T) {
	_, err := New(Config{BaseURL: "http://x", APIKey: "k"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ChannelID")
}

func TestNew_DefaultBatchSize(t *testing.T) {
	var calls int32
	fields := []fieldEntry{{Index: 1, Name: "temperature", Type: "float"}}
	_, cfg := setupTestServer(t, fields, 1, http.StatusCreated, &calls)
	cfg.BatchSize = 0

	client, err := New(cfg)
	require.NoError(t, err)
	assert.Equal(t, defaultBatchSize, client.cfg.BatchSize)
}

func TestNew_SchemaFetchFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	_, err := New(Config{
		BaseURL:    srv.URL,
		APIKey:     "bad",
		ChannelID:  "c",
		ConfigFile: filepath.Join(tmpDir, "sdk.json"),
	})
	require.Error(t, err)
}

func TestSend_Success(t *testing.T) {
	var calls int32
	fields := []fieldEntry{{Index: 1, Name: "temperature", Type: "float"}}
	_, cfg := setupTestServer(t, fields, 1, http.StatusCreated, &calls)

	client, err := New(cfg)
	require.NoError(t, err)

	client.SetField("temperature", 28.5)
	require.NoError(t, client.Send(context.Background()))
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestSend_EmptyBatch_NoRequest(t *testing.T) {
	var calls int32
	fields := []fieldEntry{{Index: 1, Name: "temperature", Type: "float"}}
	_, cfg := setupTestServer(t, fields, 1, http.StatusCreated, &calls)

	client, err := New(cfg)
	require.NoError(t, err)

	// No SetField calls — Send should be a no-op.
	require.NoError(t, client.Send(context.Background()))
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls))
}

func TestSend_409_RetriesAfterSchemaRefetch(t *testing.T) {
	fields := []fieldEntry{{Index: 1, Name: "temperature", Type: "float"}}
	var ingestCalls int32
	var schemaCalls int32
	// First ingest returns 409, second returns 201.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/channels/test-chan/schema":
			atomic.AddInt32(&schemaCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(schemaEnvelope{
				Success: true,
				Data:    schemaData{Fields: fields, SchemaVersion: 2},
			})
		case "/v1/channels/test-chan/data":
			n := atomic.AddInt32(&ingestCalls, 1)
			if n == 1 {
				w.WriteHeader(http.StatusConflict) // 409 on first attempt
			} else {
				w.WriteHeader(http.StatusCreated)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	cfg := Config{
		BaseURL:    srv.URL,
		APIKey:     "test-key",
		ChannelID:  "test-chan",
		ConfigFile: filepath.Join(tmpDir, "sdk.json"),
	}
	client, err := New(cfg)
	require.NoError(t, err)

	// schemaCalls == 1 after New()
	client.SetField("temperature", 28.5)
	require.NoError(t, client.Send(context.Background()))

	// Should have called ingest twice (409 + retry), schema twice (init + re-fetch).
	assert.Equal(t, int32(2), atomic.LoadInt32(&ingestCalls))
	assert.Equal(t, int32(2), atomic.LoadInt32(&schemaCalls))
}

func TestSend_UnexpectedStatus(t *testing.T) {
	var calls int32
	fields := []fieldEntry{{Index: 1, Name: "temperature", Type: "float"}}
	_, cfg := setupTestServer(t, fields, 1, http.StatusServiceUnavailable, &calls)

	client, err := New(cfg)
	require.NoError(t, err)

	client.SetField("temperature", 28.5)
	err = client.Send(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestSend_RecommendedFormatPersisted(t *testing.T) {
	fields := []fieldEntry{{Index: 1, Name: "temperature", Type: "float"}}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/channels/test-chan/schema", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(schemaEnvelope{
			Success: true,
			Data:    schemaData{Fields: fields, SchemaVersion: 1},
		})
	})
	mux.HandleFunc("/v1/channels/test-chan/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Recommended-Format", "msgpack")
		w.WriteHeader(http.StatusCreated)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "sdk.json")
	cfg := Config{
		BaseURL:    srv.URL,
		APIKey:     "test-key",
		ChannelID:  "test-chan",
		ConfigFile: cfgFile,
	}

	client, err := New(cfg)
	require.NoError(t, err)
	client.SetField("temperature", 28.5)
	require.NoError(t, client.Send(context.Background()))

	// Check persisted config.
	lc, err := loadConfig(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "msgpack", lc.Format)
}

func TestSetFieldAt_UsesProvidedTime(t *testing.T) {
	var calls int32
	fields := []fieldEntry{{Index: 1, Name: "temperature", Type: "float"}}
	_, cfg := setupTestServer(t, fields, 1, http.StatusCreated, &calls)

	client, err := New(cfg)
	require.NoError(t, err)

	ts := time.Unix(1700000000, 0).UTC()
	client.SetFieldAt("temperature", 30.0, ts)
	require.Len(t, client.pending, 1)
	assert.Equal(t, ts, client.pending[0].ts)
}

func TestNew_PersistsConfig(t *testing.T) {
	var calls int32
	fields := []fieldEntry{{Index: 1, Name: "temperature", Type: "float"}}
	_, cfg := setupTestServer(t, fields, 5, http.StatusCreated, &calls)

	_, err := New(cfg)
	require.NoError(t, err)

	lc, err := loadConfig(cfg.ConfigFile)
	require.NoError(t, err)
	assert.Equal(t, "test-chan", lc.ChannelID)
	assert.Equal(t, uint32(5), lc.SchemaVersion)
	assert.Equal(t, "msgpack", lc.Format)
}

func TestNew_LoadsExistingConfig(t *testing.T) {
	var calls int32
	fields := []fieldEntry{{Index: 1, Name: "temperature", Type: "float"}}
	_, cfg := setupTestServer(t, fields, 1, http.StatusCreated, &calls)

	// Pre-populate config file.
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg.ConfigFile), 0o700))
	existing := localConfig{ChannelID: "test-chan", SchemaVersion: 1, Format: "ojson"}
	data, _ := json.Marshal(existing)
	require.NoError(t, os.WriteFile(cfg.ConfigFile, data, 0o600))

	client, err := New(cfg)
	require.NoError(t, err)
	// After New, schema version is refreshed from server (1), format was "ojson"
	// but gets overwritten to current format after schema fetch.
	assert.NotNil(t, client)
}
