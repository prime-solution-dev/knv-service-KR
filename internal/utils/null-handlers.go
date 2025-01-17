package utils

import (
	"slices"
	"strconv"
	"strings"
	"time"
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
		case "datetime":
			if v, ok := val.(time.Time); ok {
				return v
			}

			shouldAddYear := []string{
				"02 Jan 15:04",
			}

			if strVal, ok := val.(string); ok {
				layouts := []string{
					"2006-01-02",
					"2006-01-02 15:04:05",
					"2006-01-02T15:04:05Z",
					"02 Jan 15:04",
				}
				for _, layout := range layouts {
					if parsedTime, err := time.Parse(layout, strVal); err == nil {
						if slices.Contains(shouldAddYear, layout) {
							return time.Date(time.Now().Year(), parsedTime.Month(), parsedTime.Day(), parsedTime.Hour(), parsedTime.Minute(), 0, 0, &time.Location{})
						}

						return parsedTime
					}
				}
			}
			return time.Time{}
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
	case "datetime":
		return time.Time{}
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
