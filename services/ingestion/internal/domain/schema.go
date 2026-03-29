package domain

// FieldEntry describes one field in the compact-format index map.
type FieldEntry struct {
	Index uint8
	Name  string
	Type  string // "float", "integer", "string", "boolean"
}

// DeviceSchema holds the auth result plus field schema for compact format deserialization.
type DeviceSchema struct {
	DeviceID      string
	ChannelID     string
	Fields        []FieldEntry
	SchemaVersion uint32
}
