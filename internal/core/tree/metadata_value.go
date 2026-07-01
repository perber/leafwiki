package tree

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// MetadataValue is a typed, recursive node in the page metadata tree.
// Scalar types use Value; list uses Items; object uses Fields.
type MetadataValue struct {
	Type   string                   `json:"type"`
	Value  string                   `json:"value,omitempty"`
	Items  []MetadataValue          `json:"items,omitempty"`
	Fields map[string]MetadataValue `json:"fields,omitempty"`
}

// Allowed Type values for MetadataValue.
const (
	MetadataTypeText     = "text"
	MetadataTypeNumber   = "number"
	MetadataTypeBoolean  = "boolean"
	MetadataTypeDate     = "date"
	MetadataTypeDatetime = "datetime"
	MetadataTypeNull     = "null"
	MetadataTypeList     = "list"
	MetadataTypeObject   = "object"
)

// IsValidMetadataType reports whether t is an allowed MetadataValue type.
func IsValidMetadataType(t string) bool {
	switch t {
	case MetadataTypeText, MetadataTypeNumber, MetadataTypeBoolean,
		MetadataTypeDate, MetadataTypeDatetime, MetadataTypeNull,
		MetadataTypeList, MetadataTypeObject:
		return true
	}
	return false
}

// IsScalar reports whether the value is a scalar type (not list or object).
func (m MetadataValue) IsScalar() bool {
	switch m.Type {
	case MetadataTypeList, MetadataTypeObject:
		return false
	}
	return true
}

// reDateOnly matches ISO date strings like 2024-01-15.
var reDateOnly = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// YamlValueToMetadataValue converts a raw value from YAML frontmatter
// (as returned by yaml.v3 into interface{}) to a typed MetadataValue.
func YamlValueToMetadataValue(v interface{}) (MetadataValue, error) {
	switch val := v.(type) {
	case nil:
		return MetadataValue{Type: MetadataTypeNull}, nil

	case bool:
		s := "false"
		if val {
			s = "true"
		}
		return MetadataValue{Type: MetadataTypeBoolean, Value: s}, nil

	case int:
		return MetadataValue{Type: MetadataTypeNumber, Value: strconv.Itoa(val)}, nil

	case int64:
		return MetadataValue{Type: MetadataTypeNumber, Value: strconv.FormatInt(val, 10)}, nil

	case float64:
		return MetadataValue{Type: MetadataTypeNumber, Value: strconv.FormatFloat(val, 'f', -1, 64)}, nil

	case string:
		return metadataValueFromString(val), nil

	case time.Time:
		return metadataValueFromTime(val), nil

	case []interface{}:
		items := make([]MetadataValue, 0, len(val))
		for i, item := range val {
			child, err := YamlValueToMetadataValue(item)
			if err != nil {
				return MetadataValue{}, fmt.Errorf("items[%d]: %w", i, err)
			}
			items = append(items, child)
		}
		return MetadataValue{Type: MetadataTypeList, Items: items}, nil

	case map[string]interface{}:
		fields := make(map[string]MetadataValue, len(val))
		for k, fv := range val {
			child, err := YamlValueToMetadataValue(fv)
			if err != nil {
				return MetadataValue{}, fmt.Errorf("field %q: %w", k, err)
			}
			fields[k] = child
		}
		return MetadataValue{Type: MetadataTypeObject, Fields: fields}, nil

	default:
		// Fallback: stringify unknown types.
		return MetadataValue{Type: MetadataTypeText, Value: fmt.Sprintf("%v", val)}, nil
	}
}

func metadataValueFromString(s string) MetadataValue {
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return MetadataValue{Type: MetadataTypeDatetime, Value: s}
	}
	// Try bare date.
	if reDateOnly.MatchString(s) {
		if _, err := time.Parse("2006-01-02", s); err == nil {
			return MetadataValue{Type: MetadataTypeDate, Value: s}
		}
	}
	return MetadataValue{Type: MetadataTypeText, Value: s}
}

func metadataValueFromTime(ts time.Time) MetadataValue {
	ts = ts.UTC()
	// YAML parses bare dates (e.g. 2024-01-15) as time.Time at midnight UTC
	// with a zero time component.
	if ts.Hour() == 0 && ts.Minute() == 0 && ts.Second() == 0 && ts.Nanosecond() == 0 {
		return MetadataValue{Type: MetadataTypeDate, Value: ts.Format("2006-01-02")}
	}
	return MetadataValue{Type: MetadataTypeDatetime, Value: ts.Format(time.RFC3339)}
}

// metadataValueToYAML converts a MetadataValue back to a Go value
// suitable for YAML serialization via yaml.v3.
func metadataValueToYAML(v MetadataValue) (interface{}, error) {
	switch v.Type {
	case MetadataTypeText:
		return v.Value, nil

	case MetadataTypeDate:
		return v.Value, nil

	case MetadataTypeDatetime:
		return v.Value, nil

	case MetadataTypeNull:
		return nil, nil

	case MetadataTypeNumber:
		// Try integer first, then float.
		if i, err := strconv.ParseInt(v.Value, 10, 64); err == nil {
			return i, nil
		}
		f, err := strconv.ParseFloat(v.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number value %q: %w", v.Value, err)
		}
		return f, nil

	case MetadataTypeBoolean:
		b, err := strconv.ParseBool(v.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid boolean value %q: %w", v.Value, err)
		}
		return b, nil

	case MetadataTypeList:
		result := make([]interface{}, 0, len(v.Items))
		for i, item := range v.Items {
			child, err := metadataValueToYAML(item)
			if err != nil {
				return nil, fmt.Errorf("items[%d]: %w", i, err)
			}
			result = append(result, child)
		}
		return result, nil

	case MetadataTypeObject:
		result := make(map[string]interface{}, len(v.Fields))
		for k, fv := range v.Fields {
			child, err := metadataValueToYAML(fv)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", k, err)
			}
			result[k] = child
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unknown MetadataValue type %q", v.Type)
	}
}
