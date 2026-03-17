package notify

import (
	"fmt"
	"strconv"
	"strings"
)

func asString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func asInt(value any) int {
	switch actual := value.(type) {
	case int:
		return actual
	case int64:
		return int(actual)
	case float64:
		return int(actual)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(actual))
		return parsed
	default:
		return 0
	}
}

func splitCommaValues(value string) []string {
	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func validateRequiredConfig(config map[string]any, fields ...string) error {
	for _, field := range fields {
		if strings.TrimSpace(asString(config[field])) == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	return nil
}
