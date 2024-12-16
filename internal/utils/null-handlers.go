package utils

import (
	"strconv"
	"strings"
)

func GetStringValueOrNil(row map[string]interface{}, key string) *string {
	if val, ok := row[key]; ok {
		if strVal, ok := val.(string); ok {
			return &strVal
		}
	}
	return nil
}

func GetDefaultValue(row map[string]interface{}, key string, defaultType string) interface{} {
	if val, ok := row[key]; ok {
		switch defaultType {
		case "string":
			if v, ok := val.(string); ok && v != "" {
				return v
			}
			return ""
		case "float64":
			if strVal, ok := val.(string); ok {
				strVal = strings.TrimSpace(strVal)
				strVal = strings.ReplaceAll(strVal, ",", "")

				if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
					return floatVal
				}
			}
			if v, ok := val.(float64); ok {
				return v
			}
			return 0.0
		case "int":
			if v, ok := val.(int); ok {
				return v
			}
			return 0
		case "int64":
			if v, ok := val.(int64); ok {
				return v
			}
			return int64(0)
		default:
			return nil
		}
	}

	switch defaultType {
	case "string":
		return ""
	case "float64":
		return 0.0
	case "int":
		return 0
	case "int64":
		return int64(0)
	default:
		return nil
	}
}

func NullHandler(input *string) string {
	if input == nil {
		return ""
	}
	return *input
}
