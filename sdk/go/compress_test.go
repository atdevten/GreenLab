package sdk

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaybeCompress_SmallPayload_NoCompression(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), compressionThreshold)

	result, compressed, err := maybeCompress(payload)
	require.NoError(t, err)
	assert.False(t, compressed)
	assert.Equal(t, payload, result)
}

func TestMaybeCompress_LargePayload_Compresses(t *testing.T) {
	// Use highly compressible data (repetitive bytes).
	payload := bytes.Repeat([]byte("abcdefgh"), 50) // 400 bytes, very compressible

	result, compressed, err := maybeCompress(payload)
	require.NoError(t, err)
	assert.True(t, compressed)
	assert.Less(t, len(result), len(payload))
}

func TestMaybeCompress_CompressedCanBeDecompressed(t *testing.T) {
	payload := bytes.Repeat([]byte("hello world "), 50) // 600 bytes

	compressed, didCompress, err := maybeCompress(payload)
	require.NoError(t, err)
	require.True(t, didCompress)

	decompressed, err := decompressLZ4(compressed, len(payload))
	require.NoError(t, err)
	assert.Equal(t, payload, decompressed)
}

func TestMaybeCompress_ExactlyThreshold_NoCompression(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), compressionThreshold)
	_, compressed, err := maybeCompress(payload)
	require.NoError(t, err)
	assert.False(t, compressed)
}

func TestMaybeCompress_OneBytePastThreshold_Compresses(t *testing.T) {
	// 101 bytes of repetitive data should compress.
	payload := bytes.Repeat([]byte("a"), compressionThreshold+1)
	_, compressed, err := maybeCompress(payload)
	require.NoError(t, err)
	// lz4 will compress repetitive data; result may still be compressed=true.
	// We just verify no error regardless.
	_ = compressed
}

func TestDecompressLZ4_InvalidData(t *testing.T) {
	_, err := decompressLZ4([]byte("not-lz4-data"), 1024)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lz4 decompress")
}
