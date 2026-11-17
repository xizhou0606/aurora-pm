package docsource

import (
	"encoding/json"
	"fmt"
)

// DiscloseUnexpanded makes incomplete coverage visible even if a model omits it.
func DiscloseUnexpanded(output json.RawMessage, input map[string]any) json.RawMessage {
	raw, err := json.Marshal(input["sources"])
	if err != nil {
		return output
	}
	var sources []Snapshot
	if json.Unmarshal(raw, &sources) != nil {
		return output
	}
	var notices []string
	for i, source := range sources {
		if len(source.Unexpanded) > 0 {
			notices = append(notices, fmt.Sprintf("来源 sources[%d] 含 %d 处未展开资源（图片、外链或嵌入内容）；本次分析未覆盖其内容。", i, len(source.Unexpanded)))
		}
	}
	if len(notices) == 0 {
		return output
	}
	var result map[string]any
	if json.Unmarshal(output, &result) != nil {
		return output
	}
	risks, _ := result["risks"].([]any)
	for _, notice := range notices {
		risks = append(risks, notice)
	}
	result["risks"] = risks
	encoded, err := json.Marshal(result)
	if err != nil {
		return output
	}
	return encoded
}
