package deviceregistry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/ingestion/internal/domain"
)

func TestClient_GetByAPIKey(t *testing.T) {
	ctx := context.Background()

	t.Run("200 response returns correct DeviceSchema", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/internal/validate-api-key", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "test-api-key", body["api_key"])
			assert.Equal(t, "chan-uuid", body["channel_id"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"device_id": "dev-uuid-1",
					"fields": []map[string]any{
						{"index": 1, "name": "temperature", "type": "float"},
						{"index": 2, "name": "humidity", "type": "float"},
					},
					"schema_version": 1,
				},
			})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, srv.Client())
		schema, err := client.GetByAPIKey(ctx, "test-api-key", "chan-uuid")
		require.NoError(t, err)

		assert.Equal(t, "dev-uuid-1", schema.DeviceID)
		assert.Equal(t, "chan-uuid", schema.ChannelID)
		assert.Equal(t, uint32(1), schema.SchemaVersion)
		require.Len(t, schema.Fields, 2)
		assert.Equal(t, domain.FieldEntry{Index: 1, Name: "temperature", Type: "float"}, schema.Fields[0])
		assert.Equal(t, domain.FieldEntry{Index: 2, Name: "humidity", Type: "float"}, schema.Fields[1])
	})

	t.Run("401 response returns ErrDeviceNotFound", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, srv.Client())
		_, err := client.GetByAPIKey(ctx, "bad-key", "chan-uuid")
		assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
	})

	t.Run("500 response returns wrapped error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, srv.Client())
		_, err := client.GetByAPIKey(ctx, "key", "chan")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("network error returns wrapped error", func(t *testing.T) {
		client := NewClient("http://127.0.0.1:1", nil) // unreachable port
		_, err := client.GetByAPIKey(ctx, "key", "chan")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Client.GetByAPIKey")
	})

	t.Run("response with empty device_id returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"device_id": "",
					"fields":    []any{},
				},
			})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, srv.Client())
		_, err := client.GetByAPIKey(ctx, "key", "chan")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty device_id")
	})

	t.Run("malformed JSON response returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{invalid json`))
		}))
		defer srv.Close()

		client := NewClient(srv.URL, srv.Client())
		_, err := client.GetByAPIKey(ctx, "key", "chan")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})
}
