package designreviewgorm

import (
	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func completionMetadata(metadata designreview.AnalysisRunMetadata) (map[string]any, error) {
	if !metadata.Valid() {
		return nil, designreview.ErrInvalid
	}
	model := metadata.Model
	reasoningEffort := metadata.ReasoningEffort
	promptVersion := metadata.PromptVersion
	responseID := metadata.ResponseID
	durationMS := metadata.DurationMS
	return map[string]any{
		"model":            &model,
		"reasoning_effort": &reasoningEffort,
		"prompt_version":   &promptVersion,
		"response_id":      &responseID,
		"duration_ms":      &durationMS,
	}, nil
}

func copyCompletionMetadata(target, metadata map[string]any, durationKey string) {
	for key, value := range metadata {
		if key == "duration_ms" {
			if durationKey != "" {
				target[durationKey] = value
			}
			continue
		}
		target[key] = value
	}
}
