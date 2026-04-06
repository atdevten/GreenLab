package sdk

import (
	"fmt"

	"github.com/pierrec/lz4/v4"
)

const compressionThreshold = 100

// maybeCompress LZ4-compresses payload if it exceeds compressionThreshold bytes.
// Returns the (possibly compressed) payload and a boolean indicating whether
// compression was applied.
func maybeCompress(payload []byte) ([]byte, bool, error) {
	if len(payload) <= compressionThreshold {
		return payload, false, nil
	}

	buf := make([]byte, lz4.CompressBlockBound(len(payload)))
	n, err := lz4.CompressBlock(payload, buf, nil)
	if err != nil {
		return nil, false, fmt.Errorf("sdk: lz4 compress: %w", err)
	}
	if n == 0 {
		// Incompressible — send uncompressed.
		return payload, false, nil
	}
	return buf[:n], true, nil
}

// decompressLZ4 decompresses an LZ4 block-format payload.
// dst must be large enough to hold the decompressed data.
func decompressLZ4(src []byte, dstSize int) ([]byte, error) {
	dst := make([]byte, dstSize)
	n, err := lz4.UncompressBlock(src, dst)
	if err != nil {
		return nil, fmt.Errorf("sdk: lz4 decompress: %w", err)
	}
	return dst[:n], nil
}
