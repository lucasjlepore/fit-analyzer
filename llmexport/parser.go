package llmexport

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/tormoder/fit/dyncrc16"
)

const (
	compressedHeaderMask       = 0x80
	compressedLocalMesgNumMask = 0x60
	compressedTimeMask         = 0x1F
	mesgDefinitionMask         = 0x40
	devDataMask                = 0x20
	localMesgNumMask           = 0x0F

	headerSizeNoCRC = 12
	headerSizeCRC   = 14
)

type baseType uint8

const (
	baseEnum    baseType = 0x00
	baseSint8   baseType = 0x01
	baseUint8   baseType = 0x02
	baseSint16  baseType = 0x83
	baseUint16  baseType = 0x84
	baseSint32  baseType = 0x85
	baseUint32  baseType = 0x86
	baseString  baseType = 0x07
	baseFloat32 baseType = 0x88
	baseFloat64 baseType = 0x89
	baseUint8z  baseType = 0x0A
	baseUint16z baseType = 0x8B
	baseUint32z baseType = 0x8C
	baseByte    baseType = 0x0D
	baseSint64  baseType = 0x8E
	baseUint64  baseType = 0x8F
	baseUint64z baseType = 0x90
)

type baseSpec struct {
	name          string
	size          int
	signed        bool
	floating      bool
	zeroIsInvalid bool
}

var baseSpecs = map[baseType]baseSpec{
	baseEnum:    {name: "enum", size: 1},
	baseSint8:   {name: "sint8", size: 1, signed: true},
	baseUint8:   {name: "uint8", size: 1},
	baseSint16:  {name: "sint16", size: 2, signed: true},
	baseUint16:  {name: "uint16", size: 2},
	baseSint32:  {name: "sint32", size: 4, signed: true},
	baseUint32:  {name: "uint32", size: 4},
	baseString:  {name: "string", size: 1},
	baseFloat32: {name: "float32", size: 4, signed: true, floating: true},
	baseFloat64: {name: "float64", size: 8, signed: true, floating: true},
	baseUint8z:  {name: "uint8z", size: 1, zeroIsInvalid: true},
	baseUint16z: {name: "uint16z", size: 2, zeroIsInvalid: true},
	baseUint32z: {name: "uint32z", size: 4, zeroIsInvalid: true},
	baseByte:    {name: "byte", size: 1},
	baseSint64:  {name: "sint64", size: 8, signed: true},
	baseUint64:  {name: "uint64", size: 8},
	baseUint64z: {name: "uint64z", size: 8, zeroIsInvalid: true},
}

type fieldDefState struct {
	fieldNumber uint8
	size        uint8
	baseRaw     uint8
	base        baseType
}

type devFieldDefState struct {
	fieldNumber      uint8
	size             uint8
	developerDataIdx uint8
}

type localDefinitionState struct {
	localMessageType uint8
	globalMessageNum uint16
	archByte         uint8
	arch             binary.ByteOrder
	fields           []fieldDefState
	devFields        []devFieldDefState
}

type parseState struct {
	dataOffset     int
	fileData       []byte
	definitions    map[uint8]localDefinitionState
	lastTimestamp  uint32
	lastTimeOffset int32
	records        []RecordEnvelope
}

type parseOutput struct {
	Header             HeaderInfo
	HeaderCRC          CRCCheck
	FileCRC            CRCCheck
	Records            []RecordEnvelope
	DefinitionCount    int
	DataMessageCount   int
	StoredFileCRC      uint16
	ComputedFileCRC    uint16
	LeftoverBytesCount int64
}

func parseFITBytes(data []byte) (*parseOutput, error) {
	if len(data) < headerSizeNoCRC+2 {
		return nil, fmt.Errorf("fit file too short: %d bytes", len(data))
	}

	header, headerCRC, dataStart, dataSize, err := parseHeader(data)
	if err != nil {
		return nil, err
	}

	required := int(dataStart) + int(dataSize) + 2
	if len(data) < required {
		return nil, fmt.Errorf("fit file truncated: have %d bytes, need at least %d", len(data), required)
	}

	dataSection := data[dataStart : dataStart+dataSize]
	crcBytes := data[dataStart+dataSize : dataStart+dataSize+2]
	storedFileCRC := binary.LittleEndian.Uint16(crcBytes)
	computedFileCRC := dyncrc16.Checksum(data[:dataStart+dataSize])
	fileCRC := CRCCheck{
		Present:         true,
		StoredHex:       fmt.Sprintf("0x%04X", storedFileCRC),
		ComputedHex:     fmt.Sprintf("0x%04X", computedFileCRC),
		Valid:           storedFileCRC == computedFileCRC,
		ValidationStyle: "header_plus_data_checksum_equals_stored_crc",
	}

	ps := &parseState{
		dataOffset:  int(dataStart),
		fileData:    dataSection,
		definitions: make(map[uint8]localDefinitionState),
	}
	if err := ps.parseRecords(); err != nil {
		return nil, err
	}

	leftover := int64(len(data) - required)
	return &parseOutput{
		Header:             header,
		HeaderCRC:          headerCRC,
		FileCRC:            fileCRC,
		Records:            ps.records,
		DefinitionCount:    countRecordKind(ps.records, "definition"),
		DataMessageCount:   countRecordKind(ps.records, "data"),
		StoredFileCRC:      storedFileCRC,
		ComputedFileCRC:    computedFileCRC,
		LeftoverBytesCount: leftover,
	}, nil
}

func parseHeader(data []byte) (HeaderInfo, CRCCheck, uint32, uint32, error) {
	size := data[0]
	if size != headerSizeNoCRC && size != headerSizeCRC {
		return HeaderInfo{}, CRCCheck{}, 0, 0, fmt.Errorf("invalid fit header size: %d", size)
	}
	if len(data) < int(size) {
		return HeaderInfo{}, CRCCheck{}, 0, 0, fmt.Errorf("truncated fit header: need %d bytes", size)
	}

	h := HeaderInfo{
		Size:            size,
		ProtocolVersion: data[1],
		ProfileVersion:  binary.LittleEndian.Uint16(data[2:4]),
		DataSize:        binary.LittleEndian.Uint32(data[4:8]),
		DataType:        string(data[8:12]),
	}
	if h.DataType != ".FIT" {
		return HeaderInfo{}, CRCCheck{}, 0, 0, fmt.Errorf("invalid fit data type in header: %q", h.DataType)
	}

	headerCRC := CRCCheck{
		Present:         size == headerSizeCRC,
		ValidationStyle: "fit_header_crc16",
		Valid:           true,
	}
	if size == headerSizeCRC {
		stored := binary.LittleEndian.Uint16(data[12:14])
		headerCRC.StoredHex = fmt.Sprintf("0x%04X", stored)
		if stored != 0 {
			computed := dyncrc16.Checksum(data[:12])
			headerCRC.ComputedHex = fmt.Sprintf("0x%04X", computed)
			headerCRC.Valid = stored == computed
		}
	}

	return h, headerCRC, uint32(size), h.DataSize, nil
}

func (ps *parseState) parseRecords() error {
	pos := 0
	recordIndex := 0
	for pos < len(ps.fileData) {
		recordIndex++
		start := pos
		headerByte := ps.fileData[pos]
		pos++

		switch {
		case (headerByte & compressedHeaderMask) == compressedHeaderMask:
			local := (headerByte & compressedLocalMesgNumMask) >> 5
			def, ok := ps.definitions[local]
			if !ok {
				return fmt.Errorf("missing definition for compressed data message local=%d record=%d", local, recordIndex)
			}
			record, newPos, err := ps.parseDataRecord(recordIndex, start, pos, headerByte, local, def, true)
			if err != nil {
				return err
			}
			ps.records = append(ps.records, record)
			pos = newPos
		case (headerByte & mesgDefinitionMask) == mesgDefinitionMask:
			record, def, newPos, err := ps.parseDefinitionRecord(recordIndex, start, pos, headerByte)
			if err != nil {
				return err
			}
			ps.definitions[def.localMessageType] = def
			ps.records = append(ps.records, record)
			pos = newPos
		default:
			local := headerByte & localMesgNumMask
			def, ok := ps.definitions[local]
			if !ok {
				return fmt.Errorf("missing definition for data message local=%d record=%d", local, recordIndex)
			}
			record, newPos, err := ps.parseDataRecord(recordIndex, start, pos, headerByte, local, def, false)
			if err != nil {
				return err
			}
			ps.records = append(ps.records, record)
			pos = newPos
		}
	}

	if pos != len(ps.fileData) {
		return fmt.Errorf("fit parse did not consume all data bytes: consumed %d of %d", pos, len(ps.fileData))
	}
	return nil
}

func (ps *parseState) parseDefinitionRecord(recordIndex, startOffset, pos int, headerByte uint8) (RecordEnvelope, localDefinitionState, int, error) {
	read := func(n int) ([]byte, error) {
		if pos+n > len(ps.fileData) {
			return nil, fmt.Errorf("definition record truncated at byte %d", startOffset)
		}
		out := ps.fileData[pos : pos+n]
		pos += n
		return out, nil
	}

	local := headerByte & localMesgNumMask
	if _, err := read(1); err != nil { // reserved
		return RecordEnvelope{}, localDefinitionState{}, 0, err
	}

	archRaw, err := read(1)
	if err != nil {
		return RecordEnvelope{}, localDefinitionState{}, 0, err
	}
	archByte := archRaw[0]
	var (
		archLabel string
		arch      binary.ByteOrder
	)
	switch archByte {
	case 0:
		archLabel = "little"
		arch = binary.LittleEndian
	case 1:
		archLabel = "big"
		arch = binary.BigEndian
	default:
		return RecordEnvelope{}, localDefinitionState{}, 0, fmt.Errorf("invalid architecture byte %d at record %d", archByte, recordIndex)
	}

	globalBytes, err := read(2)
	if err != nil {
		return RecordEnvelope{}, localDefinitionState{}, 0, err
	}
	globalMsgNum := arch.Uint16(globalBytes)

	numFieldsRaw, err := read(1)
	if err != nil {
		return RecordEnvelope{}, localDefinitionState{}, 0, err
	}
	numFields := int(numFieldsRaw[0])

	fieldDefs := make([]FieldDefinition, 0, numFields)
	stateFields := make([]fieldDefState, 0, numFields)
	for i := 0; i < numFields; i++ {
		rawDef, err := read(3)
		if err != nil {
			return RecordEnvelope{}, localDefinitionState{}, 0, err
		}
		fieldNum := rawDef[0]
		size := rawDef[1]
		baseRaw := rawDef[2]
		bt := decompressBaseType(baseRaw)
		fieldDefs = append(fieldDefs, FieldDefinition{
			FieldNumber:   fieldNum,
			Size:          size,
			BaseTypeRaw:   baseRaw,
			BaseType:      makeBaseTypeInfo(bt),
			RawDefinition: hex.EncodeToString(rawDef),
		})
		stateFields = append(stateFields, fieldDefState{
			fieldNumber: fieldNum,
			size:        size,
			baseRaw:     baseRaw,
			base:        bt,
		})
	}

	devFieldDefs := make([]DeveloperFieldDefinition, 0)
	stateDevFields := make([]devFieldDefState, 0)
	if (headerByte & devDataMask) == devDataMask {
		devCountRaw, err := read(1)
		if err != nil {
			return RecordEnvelope{}, localDefinitionState{}, 0, err
		}
		devCount := int(devCountRaw[0])
		devFieldDefs = make([]DeveloperFieldDefinition, 0, devCount)
		stateDevFields = make([]devFieldDefState, 0, devCount)
		for i := 0; i < devCount; i++ {
			rawDef, err := read(3)
			if err != nil {
				return RecordEnvelope{}, localDefinitionState{}, 0, err
			}
			devFieldDefs = append(devFieldDefs, DeveloperFieldDefinition{
				FieldNumber:      rawDef[0],
				Size:             rawDef[1],
				DeveloperDataIdx: rawDef[2],
				RawDefinition:    hex.EncodeToString(rawDef),
			})
			stateDevFields = append(stateDevFields, devFieldDefState{
				fieldNumber:      rawDef[0],
				size:             rawDef[1],
				developerDataIdx: rawDef[2],
			})
		}
	}

	rawRecord := ps.fileData[startOffset:pos]
	state := localDefinitionState{
		localMessageType: local,
		globalMessageNum: globalMsgNum,
		archByte:         archByte,
		arch:             arch,
		fields:           stateFields,
		devFields:        stateDevFields,
	}

	return RecordEnvelope{
		FormatVersion:    ExportFormatVersion,
		RecordIndex:      recordIndex,
		FileOffset:       int64(ps.dataOffset + startOffset),
		HeaderByte:       headerByte,
		RecordKind:       "definition",
		LocalMessageType: local,
		GlobalMessageNum: globalMsgNum,
		Definition: &DefinitionRecord{
			ArchitectureByte:    archByte,
			Architecture:        archLabel,
			GlobalMessageNum:    globalMsgNum,
			FieldDefinitions:    fieldDefs,
			DeveloperDefinition: devFieldDefs,
		},
		RawRecordHex: hex.EncodeToString(rawRecord),
	}, state, pos, nil
}

func (ps *parseState) parseDataRecord(recordIndex, startOffset, pos int, headerByte, local uint8, def localDefinitionState, compressed bool) (RecordEnvelope, int, error) {
	read := func(n int) ([]byte, error) {
		if pos+n > len(ps.fileData) {
			return nil, fmt.Errorf("data record truncated at byte %d", startOffset)
		}
		out := ps.fileData[pos : pos+n]
		pos += n
		return out, nil
	}

	dataRecord := &DataRecord{
		Fields: make([]FieldValue, 0, len(def.fields)),
	}

	if compressed {
		offset := headerByte & compressedTimeMask
		info := &CompressedTimestampInfo{
			Offset5bit:   offset,
			HadReference: ps.lastTimestamp != 0,
		}
		if ps.lastTimestamp != 0 {
			timeOffset := int32(offset)
			ps.lastTimestamp += uint32((timeOffset - ps.lastTimeOffset) & int32(compressedTimeMask))
			ps.lastTimeOffset = timeOffset
			info.AbsoluteTimestampRaw = ps.lastTimestamp
			info.AbsoluteTimestampUTC = fitTimestampToUTC(ps.lastTimestamp).Format(time.RFC3339)
		}
		dataRecord.CompressedTimestamp = info
	}

	for i, fieldDef := range def.fields {
		raw, err := read(int(fieldDef.size))
		if err != nil {
			return RecordEnvelope{}, 0, err
		}
		value := decodeField(raw, fieldDef, def.arch)
		value.FieldIndex = i
		if fieldDef.fieldNumber == 253 {
			if ts, ok := asTimestampRaw(value.Decoded); ok {
				ps.lastTimestamp = ts
				ps.lastTimeOffset = int32(ts & compressedTimeMask)
				value.Timestamp = &TimeProjection{
					Raw: ts,
					UTC: fitTimestampToUTC(ts).Format(time.RFC3339),
				}
			}
		}
		dataRecord.Fields = append(dataRecord.Fields, value)
	}

	if len(def.devFields) > 0 {
		dataRecord.DeveloperFields = make([]DeveloperFieldValue, 0, len(def.devFields))
		for i, ddf := range def.devFields {
			raw, err := read(int(ddf.size))
			if err != nil {
				return RecordEnvelope{}, 0, err
			}
			dataRecord.DeveloperFields = append(dataRecord.DeveloperFields, DeveloperFieldValue{
				FieldIndex:        i,
				FieldNumber:       ddf.fieldNumber,
				Size:              ddf.size,
				DeveloperDataIdx:  ddf.developerDataIdx,
				RawHex:            hex.EncodeToString(raw),
				DecodedByteValues: bytesToInts(raw),
			})
		}
	}

	rawRecord := ps.fileData[startOffset:pos]
	return RecordEnvelope{
		FormatVersion:    ExportFormatVersion,
		RecordIndex:      recordIndex,
		FileOffset:       int64(ps.dataOffset + startOffset),
		HeaderByte:       headerByte,
		RecordKind:       "data",
		LocalMessageType: local,
		GlobalMessageNum: def.globalMessageNum,
		Data:             dataRecord,
		RawRecordHex:     hex.EncodeToString(rawRecord),
	}, pos, nil
}

func decodeField(raw []byte, def fieldDefState, arch binary.ByteOrder) FieldValue {
	bt := def.base
	spec, ok := baseSpecs[bt]
	if !ok {
		return FieldValue{
			FieldNumber: def.fieldNumber,
			Size:        def.size,
			BaseTypeRaw: def.baseRaw,
			BaseType: BaseTypeInfo{
				CanonicalByte: uint8(bt),
				Name:          fmt.Sprintf("unknown_0x%02X", uint8(bt)),
				SizeBytes:     1,
			},
			RawHex:      hex.EncodeToString(raw),
			Decoded:     bytesToInts(raw),
			DecodedType: "bytes",
			IsArray:     len(raw) > 1,
			Invalid:     false,
			DecodeError: "unknown base type",
		}
	}

	field := FieldValue{
		FieldNumber: def.fieldNumber,
		Size:        def.size,
		BaseTypeRaw: def.baseRaw,
		BaseType:    makeBaseTypeInfo(bt),
		RawHex:      hex.EncodeToString(raw),
	}

	if bt == baseString {
		field.DecodedType = "string"
		str := decodeNullTerminatedString(raw)
		field.Decoded = str
		field.Invalid = len(str) == 0 && allBytes(raw, 0x00)
		field.IsArray = false
		return field
	}

	if bt == baseByte {
		field.DecodedType = "bytes"
		field.Decoded = bytesToInts(raw)
		field.Invalid = allBytes(raw, 0xFF)
		field.IsArray = len(raw) > 1
		return field
	}

	if spec.size <= 0 || len(raw)%spec.size != 0 {
		field.DecodedType = "bytes"
		field.Decoded = bytesToInts(raw)
		field.IsArray = len(raw) > 1
		field.DecodeError = fmt.Sprintf("field size %d not divisible by base size %d", len(raw), spec.size)
		return field
	}

	count := len(raw) / spec.size
	values := make([]any, 0, count)
	invalidElements := make([]int, 0)
	for i := 0; i < count; i++ {
		part := raw[i*spec.size : (i+1)*spec.size]
		v, invalid := decodeSingleValue(part, bt, arch)
		values = append(values, v)
		if invalid {
			invalidElements = append(invalidElements, i)
		}
	}

	field.InvalidElements = invalidElements
	field.Invalid = len(invalidElements) == count
	if count == 1 {
		field.Decoded = values[0]
		field.DecodedType = "scalar"
		field.IsArray = false
	} else {
		field.Decoded = values
		field.DecodedType = "array"
		field.IsArray = true
	}
	return field
}

func decodeSingleValue(raw []byte, bt baseType, arch binary.ByteOrder) (any, bool) {
	switch bt {
	case baseEnum:
		v := raw[0]
		return v, v == 0xFF
	case baseSint8:
		v := int8(raw[0])
		return v, v == int8(0x7F)
	case baseUint8:
		v := raw[0]
		return v, v == 0xFF
	case baseSint16:
		v := int16(arch.Uint16(raw))
		return v, v == int16(0x7FFF)
	case baseUint16:
		v := arch.Uint16(raw)
		return v, v == 0xFFFF
	case baseSint32:
		v := int32(arch.Uint32(raw))
		return v, v == int32(0x7FFFFFFF)
	case baseUint32:
		v := arch.Uint32(raw)
		return v, v == 0xFFFFFFFF
	case baseFloat32:
		bits := arch.Uint32(raw)
		v := float64(math.Float32frombits(bits))
		return v, bits == 0xFFFFFFFF
	case baseFloat64:
		bits := arch.Uint64(raw)
		v := math.Float64frombits(bits)
		return v, bits == 0xFFFFFFFFFFFFFFFF
	case baseUint8z:
		v := raw[0]
		return v, v == 0x00
	case baseUint16z:
		v := arch.Uint16(raw)
		return v, v == 0x0000
	case baseUint32z:
		v := arch.Uint32(raw)
		return v, v == 0x00000000
	case baseSint64:
		v := int64(arch.Uint64(raw))
		return v, v == int64(0x7FFFFFFFFFFFFFFF)
	case baseUint64:
		v := arch.Uint64(raw)
		return v, v == 0xFFFFFFFFFFFFFFFF
	case baseUint64z:
		v := arch.Uint64(raw)
		return v, v == 0x0000000000000000
	default:
		return bytesToInts(raw), false
	}
}

func fitTimestampToUTC(ts uint32) time.Time {
	base := time.Date(1989, 12, 31, 0, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(ts) * time.Second)
}

func asTimestampRaw(v any) (uint32, bool) {
	switch x := v.(type) {
	case uint32:
		if x == 0xFFFFFFFF {
			return 0, false
		}
		return x, true
	case []any:
		if len(x) > 0 {
			if y, ok := x[0].(uint32); ok && y != 0xFFFFFFFF {
				return y, true
			}
		}
	}
	return 0, false
}

func makeBaseTypeInfo(bt baseType) BaseTypeInfo {
	spec, ok := baseSpecs[bt]
	if !ok {
		return BaseTypeInfo{
			CanonicalByte: uint8(bt),
			Name:          fmt.Sprintf("unknown_0x%02X", uint8(bt)),
			SizeBytes:     1,
		}
	}
	return BaseTypeInfo{
		CanonicalByte: uint8(bt),
		Name:          spec.name,
		SizeBytes:     spec.size,
		Signed:        spec.signed,
		Floating:      spec.floating,
		ZeroIsInvalid: spec.zeroIsInvalid,
	}
}

func decodeNullTerminatedString(raw []byte) string {
	for i := 0; i < len(raw); i++ {
		if raw[i] == 0x00 {
			return string(raw[:i])
		}
	}
	return string(raw)
}

func allBytes(raw []byte, value byte) bool {
	if len(raw) == 0 {
		return false
	}
	for _, b := range raw {
		if b != value {
			return false
		}
	}
	return true
}

func decompressBaseType(b byte) baseType {
	switch b & 0x1F {
	case 0x03:
		return baseSint16
	case 0x04:
		return baseUint16
	case 0x05:
		return baseSint32
	case 0x06:
		return baseUint32
	case 0x08:
		return baseFloat32
	case 0x09:
		return baseFloat64
	case 0x0B:
		return baseUint16z
	case 0x0C:
		return baseUint32z
	case 0x0E:
		return baseSint64
	case 0x0F:
		return baseUint64
	case 0x10:
		return baseUint64z
	default:
		return baseType(b & 0x1F)
	}
}

func countRecordKind(records []RecordEnvelope, kind string) int {
	count := 0
	for _, r := range records {
		if r.RecordKind == kind {
			count++
		}
	}
	return count
}

func bytesToInts(raw []byte) []int {
	out := make([]int, len(raw))
	for i := range raw {
		out[i] = int(raw[i])
	}
	return out
}
