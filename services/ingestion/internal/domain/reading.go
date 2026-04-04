package domain

import (
	"time"

	"github.com/google/uuid"
)

// Reading is a single telemetry data point.
type Reading struct {
	ChannelID string
	DeviceID  string
	Fields    map[string]float64
	Tags      map[string]string
	Timestamp time.Time
}

// ValidateChannelID returns ErrInvalidChannelID if id is not a valid UUID.
func ValidateChannelID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidChannelID
	}
	return nil
}

// ValidateTimestamp checks that ts is within an acceptable window.
// A 30-second forward allowance accommodates minor clock skew between devices and server.
// Pass maxAge = 0 to skip the past-bound check (useful in tests).
func ValidateTimestamp(ts time.Time, maxAge time.Duration) error {
	now := time.Now().UTC()
	if ts.After(now.Add(30 * time.Second)) {
		return ErrTimestampFuture
	}
	if maxAge > 0 && ts.Before(now.Add(-maxAge)) {
		return ErrTimestampTooOld
	}
	return nil
}

// ValidateReplayTimestamp checks that ts falls within the replay window.
// It rejects timestamps older than windowDays days and future timestamps beyond
// the standard 30-second clock-skew allowance.
func ValidateReplayTimestamp(ts time.Time, windowDays int) error {
	now := time.Now().UTC()
	if ts.After(now.Add(30 * time.Second)) {
		return ErrTimestampFuture
	}
	window := time.Duration(windowDays) * 24 * time.Hour
	if ts.Before(now.Add(-window)) {
		return ErrTimestampOutOfReplayWindow
	}
	return nil
}

// NewReading constructs a Reading. ts must be a non-zero UTC time;
// callers are responsible for supplying a default when no client timestamp is provided.
func NewReading(channelID, deviceID string, fields map[string]float64, tags map[string]string, ts time.Time) *Reading {
	return &Reading{
		ChannelID: channelID,
		DeviceID:  deviceID,
		Fields:    fields,
		Tags:      tags,
		Timestamp: ts,
	}
}
