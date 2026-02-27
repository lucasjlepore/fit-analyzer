package llmexport

import (
	"fmt"
	"strings"
	"time"

	"github.com/tormoder/fit"
)

type fieldSemantic struct {
	name   string
	units  string
	scaler func(decoded any) (any, bool)
}

var fitEpoch = time.Date(1989, 12, 31, 0, 0, 0, 0, time.UTC)

var semanticsByMessage = map[uint16]map[uint8]fieldSemantic{
	0: { // file_id
		0: {name: "type"},
		1: {name: "manufacturer"},
		2: {name: "product"},
		3: {name: "serial_number"},
		4: {name: "time_created", units: "s_since_fit_epoch", scaler: scaleTimestamp},
		5: {name: "number"},
		8: {name: "product_name"},
	},
	18: { // session
		253: {name: "timestamp", units: "s_since_fit_epoch", scaler: scaleTimestamp},
		2:   {name: "start_time", units: "s_since_fit_epoch", scaler: scaleTimestamp},
		7:   {name: "total_elapsed_time", units: "s", scaler: scaleBy(1000, 0)},
		8:   {name: "total_timer_time", units: "s", scaler: scaleBy(1000, 0)},
		9:   {name: "total_distance", units: "m", scaler: scaleBy(100, 0)},
		14:  {name: "avg_speed", units: "m/s", scaler: scaleBy(1000, 0)},
		15:  {name: "max_speed", units: "m/s", scaler: scaleBy(1000, 0)},
		16:  {name: "avg_heart_rate", units: "bpm"},
		17:  {name: "max_heart_rate", units: "bpm"},
		18:  {name: "avg_cadence", units: "rpm"},
		19:  {name: "max_cadence", units: "rpm"},
		20:  {name: "avg_power", units: "w"},
		21:  {name: "max_power", units: "w"},
		24:  {name: "total_calories", units: "kcal"},
		48:  {name: "normalized_power", units: "w"},
		57:  {name: "threshold_power", units: "w"},
	},
	19: { // lap
		253: {name: "timestamp", units: "s_since_fit_epoch", scaler: scaleTimestamp},
		2:   {name: "start_time", units: "s_since_fit_epoch", scaler: scaleTimestamp},
		7:   {name: "total_elapsed_time", units: "s", scaler: scaleBy(1000, 0)},
		8:   {name: "total_timer_time", units: "s", scaler: scaleBy(1000, 0)},
		9:   {name: "total_distance", units: "m", scaler: scaleBy(100, 0)},
		13:  {name: "avg_speed", units: "m/s", scaler: scaleBy(1000, 0)},
		14:  {name: "max_speed", units: "m/s", scaler: scaleBy(1000, 0)},
		15:  {name: "avg_heart_rate", units: "bpm"},
		16:  {name: "max_heart_rate", units: "bpm"},
		17:  {name: "avg_cadence", units: "rpm"},
		18:  {name: "max_cadence", units: "rpm"},
		19:  {name: "avg_power", units: "w"},
		20:  {name: "max_power", units: "w"},
		42:  {name: "total_work", units: "j"},
	},
	20: { // record
		253: {name: "timestamp", units: "s_since_fit_epoch", scaler: scaleTimestamp},
		2:   {name: "altitude", units: "m", scaler: scaleBy(5, 500)},
		3:   {name: "heart_rate", units: "bpm"},
		4:   {name: "cadence", units: "rpm"},
		5:   {name: "distance", units: "m", scaler: scaleBy(100, 0)},
		6:   {name: "speed", units: "m/s", scaler: scaleBy(1000, 0)},
		7:   {name: "power", units: "w"},
		9:   {name: "grade", units: "%", scaler: scaleBy(100, 0)},
		13:  {name: "temperature", units: "c"},
	},
	21: { // event
		253: {name: "timestamp", units: "s_since_fit_epoch", scaler: scaleTimestamp},
		0:   {name: "event"},
		1:   {name: "event_type"},
		2:   {name: "data16"},
		3:   {name: "data"},
		4:   {name: "event_group"},
	},
	26: { // workout
		4: {name: "wkt_name"},
		5: {name: "sport"},
		6: {name: "sub_sport"},
		7: {name: "num_valid_steps"},
		8: {name: "capabilities"},
	},
	27: { // workout_step
		254: {name: "message_index"},
		0:   {name: "wkt_step_name"},
		1:   {name: "duration_type"},
		2:   {name: "duration_value"},
		3:   {name: "target_type"},
		4:   {name: "target_value"},
		5:   {name: "custom_target_value_low"},
		6:   {name: "custom_target_value_high"},
		7:   {name: "intensity"},
		8:   {name: "notes"},
	},
	206: { // field_description
		0: {name: "developer_data_index"},
		1: {name: "field_definition_number"},
		2: {name: "fit_base_type_id"},
		3: {name: "field_name"},
		6: {name: "native_mesg_num"},
		7: {name: "native_field_num"},
		8: {name: "units"},
	},
	207: { // developer_data_id
		0: {name: "developer_id"},
		1: {name: "application_id"},
		2: {name: "manufacturer_id"},
		3: {name: "developer_data_index"},
		4: {name: "application_version"},
	},
}

func semanticForField(global uint16, field uint8) fieldSemantic {
	if m, ok := semanticsByMessage[global]; ok {
		if s, ok := m[field]; ok {
			return s
		}
	}
	return fieldSemantic{
		name: fmt.Sprintf("field_%d", field),
	}
}

func scaleBy(scale, offset float64) func(any) (any, bool) {
	return func(decoded any) (any, bool) {
		switch v := decoded.(type) {
		case float64:
			return (v / scale) - offset, true
		case int8:
			return (float64(v) / scale) - offset, true
		case int16:
			return (float64(v) / scale) - offset, true
		case int32:
			return (float64(v) / scale) - offset, true
		case int64:
			return (float64(v) / scale) - offset, true
		case uint8:
			return (float64(v) / scale) - offset, true
		case uint16:
			return (float64(v) / scale) - offset, true
		case uint32:
			return (float64(v) / scale) - offset, true
		case uint64:
			return (float64(v) / scale) - offset, true
		default:
			return nil, false
		}
	}
}

func scaleTimestamp(decoded any) (any, bool) {
	var raw uint32
	switch v := decoded.(type) {
	case uint32:
		raw = v
	case uint64:
		raw = uint32(v)
	default:
		return nil, false
	}
	if raw == 0xFFFFFFFF {
		return nil, false
	}
	return fitEpoch.Add(time.Duration(raw) * time.Second).UTC().Format(time.RFC3339), true
}

func invalidRuleForBase(base BaseTypeInfo) string {
	switch base.Name {
	case "enum":
		return "0xFF sentinel"
	case "sint8":
		return "0x7F sentinel"
	case "uint8":
		return "0xFF sentinel"
	case "sint16":
		return "0x7FFF sentinel"
	case "uint16":
		return "0xFFFF sentinel"
	case "sint32":
		return "0x7FFFFFFF sentinel"
	case "uint32":
		return "0xFFFFFFFF sentinel"
	case "float32":
		return "0xFFFFFFFF bit-pattern sentinel"
	case "float64":
		return "0xFFFFFFFFFFFFFFFF bit-pattern sentinel"
	case "uint8z", "uint16z", "uint32z", "uint64z":
		return "0 sentinel"
	case "byte":
		return "all bytes 0xFF sentinel"
	case "string":
		return "empty string / NUL-only"
	default:
		return "see FIT base type sentinel rules"
	}
}

func globalMessageName(global uint16) string {
	name := fmt.Sprint(fit.MesgNum(global))
	if strings.HasPrefix(name, "MesgNum(") {
		return fmt.Sprintf("global_%d", global)
	}
	return name
}
