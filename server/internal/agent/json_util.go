package agent

import "encoding/json"

// jsonUnmarshalMap 把 []byte 或 json.RawMessage 解为 map[string]any。
func jsonUnmarshalMap(data []byte, out *map[string]any) error {
	if len(data) == 0 {
		*out = map[string]any{}
		return nil
	}
	return json.Unmarshal(data, out)
}
