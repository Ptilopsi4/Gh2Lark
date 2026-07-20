package webhook

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// EventEnvelope contains the GitHub fields shared by all webhook events and a
// safely navigable representation of the full payload. It lets callers render
// less-common GitHub events without unsafe type assertions.
type EventEnvelope struct {
	Action     string         `json:"action"`
	Repository Repo           `json:"repository"`
	Sender     User           `json:"sender"`
	Raw        map[string]any `json:"-"`
}

// ParseEventEnvelope validates and decodes a GitHub webhook JSON object.
func ParseEventEnvelope(body []byte) (*EventEnvelope, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("webhook payload must be a JSON object")
	}

	var event EventEnvelope
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	event.Raw = raw
	return &event, nil
}

// Value retrieves a JSON value using a dot-separated path. Numeric segments
// address array items. Missing, null, and incompatible intermediate values
// return nil.
func (e *EventEnvelope) Value(path string) any {
	if e == nil || e.Raw == nil || path == "" {
		return nil
	}

	var value any = e.Raw
	for _, part := range strings.Split(path, ".") {
		switch current := value.(type) {
		case map[string]any:
			value = current[part]
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(current) {
				return nil
			}
			value = current[index]
		default:
			return nil
		}
		if value == nil {
			return nil
		}
	}
	return value
}

// String returns the first non-empty scalar value at one of paths.
func (e *EventEnvelope) String(paths ...string) string {
	for _, path := range paths {
		switch value := e.Value(path).(type) {
		case string:
			if value != "" {
				return value
			}
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(value)
		}
	}
	return ""
}

// Int returns the first numeric value at one of paths. JSON numbers and
// numeric strings are both supported.
func (e *EventEnvelope) Int(paths ...string) int {
	for _, path := range paths {
		switch value := e.Value(path).(type) {
		case float64:
			return int(value)
		case int:
			return value
		case string:
			if number, err := strconv.Atoi(value); err == nil {
				return number
			}
		}
	}
	return 0
}

// Bool returns the first boolean value at one of paths.
func (e *EventEnvelope) Bool(paths ...string) bool {
	for _, path := range paths {
		if value, ok := e.Value(path).(bool); ok {
			return value
		}
	}
	return false
}

// Strings converts a JSON string array at path into non-empty strings.
func (e *EventEnvelope) Strings(path string) []string {
	values, ok := e.Value(path).([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok && text != "" {
			result = append(result, text)
		}
	}
	return result
}

// RawSnippet returns a rune-limited JSON representation for diagnostics on
// events GitHub introduces after this service is released.
func (e *EventEnvelope) RawSnippet(maxRunes int) string {
	if e == nil || maxRunes <= 0 {
		return ""
	}
	body, err := json.Marshal(e.Raw)
	if err != nil {
		return ""
	}
	return truncateRunes(string(body), maxRunes)
}

func truncateRunes(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}
