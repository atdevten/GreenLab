package http

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/ingestion/internal/domain"
)

// buildBinaryFrame creates a valid binary frame for testing.
//
// Layout: VER(1) | DEVID(4) | TS(4) | FIELDMSK(1) | VALUES(N×2) | CRC16(2)
func buildBinaryFrame(t *testing.T, devIDBytes [4]byte, tsSec uint32, fieldMsk uint8, values []uint16) []byte {
	t.Helper()
	n := len(values)
	buf := make([]byte, 10+n*2+2)
	buf[0] = 1 // VER
	copy(buf[1:5], devIDBytes[:])
	binary.BigEndian.PutUint32(buf[5:9], tsSec)
	buf[9] = fieldMsk
	for i, v := range values {
		binary.BigEndian.PutUint16(buf[10+i*2:10+i*2+2], v)
	}
	crc := crc16CCITTFalse(buf[:len(buf)-2])
	binary.BigEndian.PutUint16(buf[len(buf)-2:], crc)
	return buf
}

// devIDFromUUID returns the first 4 bytes of a UUID string's hex content.
func devIDFromUUID(uuidStr string) [4]byte {
	// Remove hyphens and decode first 8 hex chars.
	clean := ""
	for _, c := range uuidStr {
		if c != '-' {
			clean += string(c)
		}
	}
	var b [4]byte
	for i := 0; i < 4; i++ {
		hi := hexVal(clean[i*2])
		lo := hexVal(clean[i*2+1])
		b[i] = byte(hi<<4 | lo) //nolint:gosec
	}
	return b
}

func TestBinaryDeserializer_Parse(t *testing.T) {
	authDeviceID := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	devID := devIDFromUUID(authDeviceID)
	tsSec := uint32(1741426620)

	d := newBinaryDeserializer(authDeviceID)
	assert.Equal(t, ctBinary, d.ContentType())

	t.Run("valid frame with field 1 (bit0) set", func(t *testing.T) {
		// FIELDMSK=0b00000001 → field index 1 present
		frame := buildBinaryFrame(t, devID, tsSec, 0b00000001, []uint16{1234})
		batch, err := d.Parse(frame)
		require.NoError(t, err)
		assert.Equal(t, float64(1234), batch.FieldValues[1])
		require.NotNil(t, batch.Timestamp)
		assert.Equal(t, time.Unix(int64(tsSec), 0).UTC(), *batch.Timestamp)
	})

	t.Run("valid frame with two fields (bits 0 and 1)", func(t *testing.T) {
		frame := buildBinaryFrame(t, devID, tsSec, 0b00000011, []uint16{100, 200})
		batch, err := d.Parse(frame)
		require.NoError(t, err)
		assert.Equal(t, float64(100), batch.FieldValues[1])
		assert.Equal(t, float64(200), batch.FieldValues[2])
	})

	t.Run("valid frame — all 8 fields present", func(t *testing.T) {
		frame := buildBinaryFrame(t, devID, tsSec, 0xFF, []uint16{1, 2, 3, 4, 5, 6, 7, 8})
		batch, err := d.Parse(frame)
		require.NoError(t, err)
		assert.Len(t, batch.FieldValues, 8)
		for i := uint8(1); i <= 8; i++ {
			assert.Equal(t, float64(i), batch.FieldValues[i])
		}
	})

	t.Run("wrong CRC returns ErrCRCMismatch", func(t *testing.T) {
		frame := buildBinaryFrame(t, devID, tsSec, 0b00000001, []uint16{1234})
		// Corrupt CRC
		frame[len(frame)-1] ^= 0xFF
		_, err := d.Parse(frame)
		assert.ErrorIs(t, err, domain.ErrCRCMismatch)
	})

	t.Run("wrong DEVID returns ErrDeviceIDMismatch", func(t *testing.T) {
		wrongDevID := [4]byte{0xDE, 0xAD, 0xBE, 0xEF}
		frame := buildBinaryFrame(t, wrongDevID, tsSec, 0b00000001, []uint16{1234})
		_, err := d.Parse(frame)
		assert.ErrorIs(t, err, domain.ErrDeviceIDMismatch)
	})

	t.Run("frame too short returns ErrInvalidFrameLength", func(t *testing.T) {
		_, err := d.Parse([]byte{0x01, 0x00, 0x00})
		assert.ErrorIs(t, err, domain.ErrInvalidFrameLength)
	})

	t.Run("frame length mismatch with FIELDMSK popcount returns ErrInvalidFrameLength", func(t *testing.T) {
		// Build a frame with FIELDMSK=0b00000011 (n=2) but only provide 1 value.
		// This means expected length=14 but we'll give 12 bytes.
		frame := buildBinaryFrame(t, devID, tsSec, 0b00000011, []uint16{100, 200})
		// Truncate — remove the last value + CRC and add only CRC.
		truncated := frame[:len(frame)-4]
		// Add a fake CRC (2 bytes) to reach binaryFrameMinLen.
		truncated = append(truncated, 0x00, 0x00)
		_, err := d.Parse(truncated)
		assert.ErrorIs(t, err, domain.ErrInvalidFrameLength)
	})

	t.Run("empty body returns ErrInvalidFrameLength", func(t *testing.T) {
		_, err := d.Parse([]byte{})
		assert.ErrorIs(t, err, domain.ErrInvalidFrameLength)
	})
}
