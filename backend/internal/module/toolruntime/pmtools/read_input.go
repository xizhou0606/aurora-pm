package pmtools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type idInput struct {
	ID uint `json:"id"`
}

type taskGetInput struct {
	TaskRef       string `json:"task_ref"`
	ID            uint   `json:"id"`
	ProjectSetID  uint   `json:"project_set_id"`
	PlanUnitID    uint   `json:"project_new_id"`
	RequirementID uint   `json:"requirement_id"`
}

type workflowTemplateListInput struct {
	SubjectType string `json:"subject_type"`
}

type workflowInstanceListInput struct {
	SubjectType string `json:"subject_type"`
	SubjectID   uint   `json:"subject_id"`
}

type listOutput[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

func makeListOutput[T any](items []T) listOutput[T] {
	if items == nil {
		items = []T{}
	}
	return listOutput[T]{Items: items, Total: len(items)}
}

func inputID(raw json.RawMessage) (idInput, error) {
	var input idInput
	if err := decodeInput(raw, &input); err != nil || input.ID == 0 {
		return idInput{}, fmt.Errorf("%w: id is required", ErrReadHandlersInvalid)
	}
	return input, nil
}

func decodeInput(raw json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: %v", ErrReadHandlersInvalid, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%w: trailing JSON", ErrReadHandlersInvalid)
	}
	return nil
}
