package llmexport

import "time"

const (
	// ExportFormatVersion identifies the on-disk schema for LLM exports.
	ExportFormatVersion = "fit_llm_jsonl_v1"
)

// ExportOptions controls export behavior.
type ExportOptions struct {
	// Overwrite allows writing into a non-empty output directory.
	Overwrite bool

	// CopySourceFile writes a byte-for-byte copy of the source FIT file to the output directory.
	CopySourceFile bool
}

// ExportResult describes generated files.
type ExportResult struct {
	OutputDir         string `json:"output_dir"`
	ManifestPath      string `json:"manifest_path"`
	RecordsPath       string `json:"records_path"`
	SourceCopyPath    string `json:"source_copy_path,omitempty"`
	RecordCount       int    `json:"record_count"`
	DefinitionCount   int    `json:"definition_count"`
	DataMessageCount  int    `json:"data_message_count"`
	SourceSHA256      string `json:"source_sha256"`
	SourceSizeBytes   int64  `json:"source_size_bytes"`
	FileCRCValid      bool   `json:"file_crc_valid"`
	HeaderCRCValid    bool   `json:"header_crc_valid"`
	ChainedDataRemain int64  `json:"chained_data_remain"`
}

// Manifest captures export metadata and pointers to exported files.
type Manifest struct {
	FormatVersion     string        `json:"format_version"`
	GeneratedAt       time.Time     `json:"generated_at"`
	SourceFile        string        `json:"source_file"`
	SourceFileName    string        `json:"source_file_name"`
	SourceSHA256      string        `json:"source_sha256"`
	SourceSizeBytes   int64         `json:"source_size_bytes"`
	Header            HeaderInfo    `json:"header"`
	HeaderCRC         CRCCheck      `json:"header_crc"`
	FileCRC           CRCCheck      `json:"file_crc"`
	RecordsPath       string        `json:"records_path"`
	RecordCount       int           `json:"record_count"`
	DefinitionCount   int           `json:"definition_count"`
	DataMessageCount  int           `json:"data_message_count"`
	LeftoverBytes     int64         `json:"leftover_bytes"`
	FileIdProjection  *FileIDInfo   `json:"file_id_projection,omitempty"`
	SchemaDescription SchemaDetails `json:"schema_description"`
}

// SchemaDetails documents the record shape for downstream applications.
type SchemaDetails struct {
	RecordType string   `json:"record_type"`
	Notes      []string `json:"notes"`
}

// HeaderInfo stores parsed FIT header values.
type HeaderInfo struct {
	Size            uint8  `json:"size"`
	ProtocolVersion uint8  `json:"protocol_version"`
	ProfileVersion  uint16 `json:"profile_version"`
	DataSize        uint32 `json:"data_size"`
	DataType        string `json:"data_type"`
}

// CRCCheck describes CRC validation results.
type CRCCheck struct {
	Present         bool   `json:"present"`
	StoredHex       string `json:"stored_hex,omitempty"`
	ComputedHex     string `json:"computed_hex,omitempty"`
	Valid           bool   `json:"valid"`
	ValidationStyle string `json:"validation_style"`
}

// FileIDInfo is a convenience projection from the file_id message.
type FileIDInfo struct {
	Type         string `json:"type"`
	Manufacturer string `json:"manufacturer"`
	Product      string `json:"product"`
	TimeCreated  string `json:"time_created,omitempty"`
	SerialNumber uint32 `json:"serial_number,omitempty"`
}

// RecordEnvelope is one JSONL line in records.jsonl.
// The stream preserves original FIT record order.
type RecordEnvelope struct {
	FormatVersion    string            `json:"format_version"`
	RecordIndex      int               `json:"record_index"`
	FileOffset       int64             `json:"file_offset"`
	HeaderByte       uint8             `json:"header_byte"`
	RecordKind       string            `json:"record_kind"` // "definition" or "data"
	LocalMessageType uint8             `json:"local_message_type"`
	GlobalMessageNum uint16            `json:"global_message_num,omitempty"`
	Definition       *DefinitionRecord `json:"definition,omitempty"`
	Data             *DataRecord       `json:"data,omitempty"`
	RawRecordHex     string            `json:"raw_record_hex"`
	Warnings         []string          `json:"warnings,omitempty"`
}

// DefinitionRecord captures a FIT definition message.
type DefinitionRecord struct {
	ArchitectureByte    uint8                      `json:"architecture_byte"`
	Architecture        string                     `json:"architecture"`
	GlobalMessageNum    uint16                     `json:"global_message_num"`
	FieldDefinitions    []FieldDefinition          `json:"field_definitions"`
	DeveloperDefinition []DeveloperFieldDefinition `json:"developer_field_definitions,omitempty"`
}

// FieldDefinition captures a standard field definition.
type FieldDefinition struct {
	FieldNumber   uint8        `json:"field_number"`
	Size          uint8        `json:"size"`
	BaseTypeRaw   uint8        `json:"base_type_raw"`
	BaseType      BaseTypeInfo `json:"base_type"`
	RawDefinition string       `json:"raw_definition_hex"`
}

// DeveloperFieldDefinition captures a developer-data field definition.
type DeveloperFieldDefinition struct {
	FieldNumber      uint8  `json:"field_number"`
	Size             uint8  `json:"size"`
	DeveloperDataIdx uint8  `json:"developer_data_index"`
	RawDefinition    string `json:"raw_definition_hex"`
}

// BaseTypeInfo describes canonical FIT base type information.
type BaseTypeInfo struct {
	CanonicalByte uint8  `json:"canonical_byte"`
	Name          string `json:"name"`
	SizeBytes     int    `json:"size_bytes"`
	Signed        bool   `json:"signed"`
	Floating      bool   `json:"floating"`
	ZeroIsInvalid bool   `json:"zero_is_invalid"`
}

// DataRecord captures a FIT data message.
type DataRecord struct {
	CompressedTimestamp *CompressedTimestampInfo `json:"compressed_timestamp,omitempty"`
	Fields              []FieldValue             `json:"fields"`
	DeveloperFields     []DeveloperFieldValue    `json:"developer_fields,omitempty"`
}

// CompressedTimestampInfo includes reconstructed timestamp state for compressed headers.
type CompressedTimestampInfo struct {
	Offset5bit           uint8  `json:"offset_5bit"`
	AbsoluteTimestampRaw uint32 `json:"absolute_timestamp_raw,omitempty"`
	AbsoluteTimestampUTC string `json:"absolute_timestamp_utc,omitempty"`
	HadReference         bool   `json:"had_reference"`
}

// FieldValue is a decoded field from a standard message field definition.
type FieldValue struct {
	FieldIndex      int             `json:"field_index"`
	FieldNumber     uint8           `json:"field_number"`
	Size            uint8           `json:"size"`
	BaseTypeRaw     uint8           `json:"base_type_raw"`
	BaseType        BaseTypeInfo    `json:"base_type"`
	RawHex          string          `json:"raw_hex"`
	Decoded         any             `json:"decoded"`
	DecodedType     string          `json:"decoded_type"`
	IsArray         bool            `json:"is_array"`
	Invalid         bool            `json:"invalid"`
	InvalidElements []int           `json:"invalid_elements,omitempty"`
	DecodeError     string          `json:"decode_error,omitempty"`
	Timestamp       *TimeProjection `json:"timestamp_projection,omitempty"`
}

// TimeProjection is attached when a field plausibly represents a FIT timestamp.
type TimeProjection struct {
	Raw uint32 `json:"raw"`
	UTC string `json:"utc"`
}

// DeveloperFieldValue is a decoded developer-data field.
type DeveloperFieldValue struct {
	FieldIndex        int    `json:"field_index"`
	FieldNumber       uint8  `json:"field_number"`
	Size              uint8  `json:"size"`
	DeveloperDataIdx  uint8  `json:"developer_data_index"`
	RawHex            string `json:"raw_hex"`
	DecodedByteValues []int  `json:"decoded_byte_values"`
}
