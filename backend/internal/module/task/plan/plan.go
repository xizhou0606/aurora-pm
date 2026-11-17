package taskplan

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const Capability = "requirement.task_plan"

type Plan struct {
	Chains []Chain `json:"chains"`
}

type Chain struct {
	Lane    string `json:"lane"`
	Summary string `json:"summary"`
	Tasks   []Item `json:"tasks"`
}

type Item struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
}

type CreatedTaskRef struct {
	ID       uint   `json:"id"`
	TaskKey  string `json:"taskKey"`
	Title    string `json:"title"`
	Lane     string `json:"lane"`
	Kind     string `json:"kind"`
	ParentID *uint  `json:"parentId,omitempty"`
}

type Output struct {
	Plan         Plan
	CreatedTasks []CreatedTaskRef
}

func ParseForSubject(capability, subjectType string, subjectID uint, raw json.RawMessage) (*Output, error) {
	if capability != Capability {
		return nil, nil
	}
	if subjectType != "requirement" || subjectID == 0 {
		return nil, fmt.Errorf("requirement task plan has an invalid subject")
	}
	var envelope struct {
		TaskPlan     *Plan            `json:"taskPlan"`
		CreatedTasks []CreatedTaskRef `json:"createdTasks"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.TaskPlan == nil {
		return nil, fmt.Errorf("requirement task plan output is missing")
	}
	for index := range envelope.TaskPlan.Chains {
		envelope.TaskPlan.Chains[index].Lane = strings.TrimSpace(envelope.TaskPlan.Chains[index].Lane)
	}
	if err := Validate(*envelope.TaskPlan); err != nil {
		return nil, err
	}
	if len(envelope.CreatedTasks) > 0 {
		if err := validateCreatedTasks(*envelope.TaskPlan, envelope.CreatedTasks); err != nil {
			return nil, err
		}
	}
	return &Output{Plan: *envelope.TaskPlan, CreatedTasks: envelope.CreatedTasks}, nil
}

func Validate(plan Plan) error {
	if len(plan.Chains) != 2 {
		return fmt.Errorf("requirement task plan must contain exactly two chains")
	}
	seen := map[string]bool{}
	for _, chain := range plan.Chains {
		lane := strings.TrimSpace(chain.Lane)
		if (lane != "frontend" && lane != "backend") || seen[lane] {
			return fmt.Errorf("requirement task plan must contain one frontend and one backend chain")
		}
		seen[lane] = true
		if strings.TrimSpace(chain.Summary) == "" || utf8.RuneCountInString(strings.TrimSpace(chain.Summary)) > 1000 || len(chain.Tasks) < 1 || len(chain.Tasks) > 8 {
			return fmt.Errorf("requirement task plan chain is incomplete")
		}
		for _, item := range chain.Tasks {
			if strings.TrimSpace(item.Title) == "" || utf8.RuneCountInString(strings.TrimSpace(item.Title)) > 180 ||
				strings.TrimSpace(item.Description) == "" || utf8.RuneCountInString(strings.TrimSpace(item.Description)) > 4000 ||
				len(item.AcceptanceCriteria) < 1 || len(item.AcceptanceCriteria) > 8 {
				return fmt.Errorf("requirement task plan item is invalid")
			}
			for _, criterion := range item.AcceptanceCriteria {
				if strings.TrimSpace(criterion) == "" || utf8.RuneCountInString(strings.TrimSpace(criterion)) > 500 {
					return fmt.Errorf("requirement task plan acceptance criterion is empty")
				}
			}
		}
	}
	return nil
}

func AttachCreatedTasks(raw json.RawMessage, refs []CreatedTaskRef) (json.RawMessage, error) {
	var output map[string]any
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, err
	}
	output["createdTasks"] = refs
	return json.Marshal(output)
}

func validateCreatedTasks(plan Plan, refs []CreatedTaskRef) error {
	expectedChildren := map[string]int{}
	for _, chain := range plan.Chains {
		expectedChildren[chain.Lane] = len(chain.Tasks)
	}
	if len(refs) != 2+expectedChildren["frontend"]+expectedChildren["backend"] {
		return fmt.Errorf("requirement task plan created task references are incomplete")
	}
	parents := map[string]uint{}
	children := map[string]int{}
	seenIDs := map[uint]bool{}
	for _, ref := range refs {
		if ref.ID == 0 || seenIDs[ref.ID] || strings.TrimSpace(ref.TaskKey) == "" || strings.TrimSpace(ref.Title) == "" ||
			(ref.Lane != "frontend" && ref.Lane != "backend") {
			return fmt.Errorf("requirement task plan created task reference is invalid")
		}
		seenIDs[ref.ID] = true
		switch ref.Kind {
		case "task":
			if ref.ParentID != nil || parents[ref.Lane] != 0 {
				return fmt.Errorf("requirement task plan created task reference is invalid")
			}
			parents[ref.Lane] = ref.ID
		case "subtask":
			if ref.ParentID == nil || *ref.ParentID == 0 {
				return fmt.Errorf("requirement task plan created task reference is invalid")
			}
			children[ref.Lane]++
		default:
			return fmt.Errorf("requirement task plan created task reference is invalid")
		}
	}
	for lane, count := range expectedChildren {
		if parents[lane] == 0 || children[lane] != count {
			return fmt.Errorf("requirement task plan created task references are incomplete")
		}
		for _, ref := range refs {
			if ref.Lane == lane && ref.Kind == "subtask" && *ref.ParentID != parents[lane] {
				return fmt.Errorf("requirement task plan created task reference is invalid")
			}
		}
	}
	return nil
}
