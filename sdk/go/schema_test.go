package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchSchema_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/channels/chan-1/schema", r.URL.Path)
		assert.Equal(t, "mykey", r.Header.Get("X-API-Key"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schemaEnvelope{
			Success: true,
			Data: schemaData{
				Fields: []fieldEntry{
					{Index: 1, Name: "temperature", Type: "float"},
					{Index: 2, Name: "humidity", Type: "float"},
				},
				SchemaVersion: 3,
			},
		})
	}))
	defer srv.Close()

	cs, err := fetchSchema(context.Background(), srv.URL, "chan-1", "mykey")
	require.NoError(t, err)

	assert.Equal(t, uint32(3), cs.schemaVersion)
	assert.Equal(t, uint8(1), cs.nameToIndex["temperature"])
	assert.Equal(t, uint8(2), cs.nameToIndex["humidity"])
	assert.Equal(t, "temperature", cs.indexToName[1])
	assert.Equal(t, "humidity", cs.indexToName[2])
	assert.Len(t, cs.orderedFields, 2)
}

func TestFetchSchema_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := fetchSchema(context.Background(), srv.URL, "chan-1", "bad-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 401")
}

func TestFetchSchema_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	_, err := fetchSchema(context.Background(), srv.URL, "chan-1", "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode schema")
}

func TestFetchSchema_SuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(schemaEnvelope{Success: false})
	}))
	defer srv.Close()

	_, err := fetchSchema(context.Background(), srv.URL, "chan-1", "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failure")
}

func TestBuildChannelSchema(t *testing.T) {
	data := schemaData{
		Fields: []fieldEntry{
			{Index: 1, Name: "temp", Type: "float"},
			{Index: 3, Name: "co2", Type: "float"},
		},
		SchemaVersion: 7,
	}

	cs := buildChannelSchema(data)

	assert.Equal(t, uint32(7), cs.schemaVersion)
	assert.Equal(t, uint8(1), cs.nameToIndex["temp"])
	assert.Equal(t, uint8(3), cs.nameToIndex["co2"])
	assert.Equal(t, "temp", cs.indexToName[1])
	assert.Equal(t, "co2", cs.indexToName[3])
}
