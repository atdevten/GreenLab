package http

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"strings"
	"time"

	"github.com/greenlab/ingestion/internal/domain"
)

// Binary frame format:
//
//	VER(1) | DEVID(4) | TS(4) | FIELDMSK(1) | VALUES(N×2) | CRC16(2)
//	Total: 12 + N*2 bytes where N = popcount(FIELDMSK)
//
// Layout (byte offsets):
//
//	[0]      VER
//	[1:5]    DEVID  (first 4 bytes of device UUID, big-endian hex)
//	[5:9]    TS     (unix seconds, uint32 big-endian)
//	[9]      FIELDMSK (8-bit bitmap; bit i set = field index i+1 present)
//	[10:10+N*2]  VALUES (N * uint16 big-endian; raw, scale/offset deferred to TODO-028)
//	[10+N*2:]    CRC16  (CRC16/CCITT-FALSE over all preceding bytes)
//
// DEVID validation: [1:5] must match the first 4 bytes of the authenticated device UUID.
const binaryFrameMinLen = 12 // VER(1)+DEVID(4)+TS(4)+FIELDMSK(1)+CRC16(2), N=0

type binaryDeserializer struct {
	authDeviceID string // device ID from auth context (used for DEVID validation)
}

func newBinaryDeserializer(authDeviceID string) *binaryDeserializer {
	return &binaryDeserializer{authDeviceID: authDeviceID}
}

func (d *binaryDeserializer) ContentType() string { return ctBinary }

func (d *binaryDeserializer) Parse(body []byte) (CompactBatch, error) {
	if len(body) < binaryFrameMinLen {
		return CompactBatch{}, fmt.Errorf("%w: frame too short (%d bytes, need at least %d)",
			domain.ErrInvalidFrameLength, len(body), binaryFrameMinLen)
	}

	fieldMsk := body[9]
	n := bits.OnesCount8(fieldMsk)
	expectedLen := binaryFrameMinLen + n*2
	if len(body) != expectedLen {
		return CompactBatch{}, fmt.Errorf("%w: expected %d bytes, got %d",
			domain.ErrInvalidFrameLength, expectedLen, len(body))
	}

	// Verify CRC16/CCITT-FALSE over all bytes before the final 2.
	payload := body[:len(body)-2]
	gotCRC := binary.BigEndian.Uint16(body[len(body)-2:])
	if crc16CCITTFalse(payload) != gotCRC {
		return CompactBatch{}, domain.ErrCRCMismatch
	}

	// Validate DEVID: bytes [1:5] of the frame must match first 4 bytes of auth device UUID.
	devIDBytes := body[1:5]
	if err := validateDevID(devIDBytes, d.authDeviceID); err != nil {
		return CompactBatch{}, err
	}

	// Decode timestamp (unix seconds, big-endian uint32).
	tsSec := binary.BigEndian.Uint32(body[5:9])
	ts := time.Unix(int64(tsSec), 0).UTC()

	// Decode field values.
	fieldValues := make(map[uint8]float64, n)
	valueOffset := 10
	for i := 0; i < 8; i++ {
		if fieldMsk&(1<<uint(i)) != 0 {
			raw := binary.BigEndian.Uint16(body[valueOffset : valueOffset+2])
			fieldValues[uint8(i+1)] = float64(raw) //nolint:gosec // i bounded 0–7
			valueOffset += 2
		}
	}

	return CompactBatch{
		FieldValues: fieldValues,
		Timestamp:   &ts,
		// SchemaVersion not included in binary frame; 0 = skip schema version check
		SchemaVersion: 0,
		DeviceID:      d.authDeviceID,
	}, nil
}

// validateDevID checks that the 4-byte DEVID in the frame matches the first 4 bytes of authDeviceID.
// authDeviceID is a UUID string like "6ba7b810-9dad-11d1-80b4-00c04fd430c8".
func validateDevID(devIDBytes []byte, authDeviceID string) error {
	// Strip hyphens and decode first 8 hex chars (= 4 bytes).
	clean := strings.ReplaceAll(authDeviceID, "-", "")
	if len(clean) < 8 {
		return fmt.Errorf("%w: auth device_id too short", domain.ErrDeviceIDMismatch)
	}
	var expected [4]byte
	for i := 0; i < 4; i++ {
		hi := hexVal(clean[i*2])
		lo := hexVal(clean[i*2+1])
		if hi < 0 || lo < 0 {
			return fmt.Errorf("%w: invalid hex in auth device_id", domain.ErrDeviceIDMismatch)
		}
		expected[i] = byte(hi<<4 | lo) //nolint:gosec
	}
	for i := 0; i < 4; i++ {
		if devIDBytes[i] != expected[i] {
			return fmt.Errorf("%w: frame DEVID does not match authenticated device", domain.ErrDeviceIDMismatch)
		}
	}
	return nil
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// crc16CCITTFalse computes CRC16/CCITT-FALSE (poly=0x1021, init=0xFFFF, refIn=false, refOut=false).
func crc16CCITTFalse(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
