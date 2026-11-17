package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

var ErrSubjectStartInvalid = errors.New("workflow subject start is invalid")

type SubjectStartCommand struct {
	WorkspaceID, PrincipalID  uint
	ApplyKey, InputDigest     string
	TemplateID                uint
	SubjectType               string
	SubjectID, SubjectVersion uint
	ProjectSetID, TeamID      uint
	SubjectTitle              string
	SubjectSnapshot           json.RawMessage
}

type SubjectStartResult struct {
	InstanceID     uint     `json:"instance_id"`
	Status         string   `json:"status"`
	SubjectStatus  string   `json:"subject_status"`
	CurrentNodeIDs []string `json:"current_node_ids"`
}

type SubjectStartStore interface {
	StartSubject(context.Context, SubjectStartCommand) (SubjectStartResult, error)
}

type SubjectAIStarter interface {
	StartSubjectAI(context.Context, SubjectAIStart) (uint, error)
}

type SubjectAIStart struct {
	WorkspaceID, PrincipalID  uint
	Capability, SubjectType   string
	SubjectID, SubjectVersion uint
	SubjectSnapshot           json.RawMessage
}

type SubjectAssignment struct {
	WorkspaceID, PrincipalID  uint
	ProjectSetID, TeamID      uint
	InstanceID, SubjectID     uint
	NodeID, NodeName          string
	AssigneeUserID            uint
	SubjectType, SubjectTitle string
}

type SubjectAssignmentWriter interface {
	RecordSubjectAssignment(context.Context, SubjectAssignment) error
}

type SubjectStartService struct{ store SubjectStartStore }

func NewSubjectStartService(store SubjectStartStore) (*SubjectStartService, error) {
	if store == nil {
		return nil, ErrSubjectStartInvalid
	}
	return &SubjectStartService{store: store}, nil
}

func (service *SubjectStartService) StartSubject(ctx context.Context, command SubjectStartCommand) (SubjectStartResult, error) {
	command.ApplyKey, command.InputDigest = strings.TrimSpace(command.ApplyKey), strings.TrimSpace(command.InputDigest)
	command.SubjectType, command.SubjectTitle = strings.TrimSpace(command.SubjectType), strings.TrimSpace(command.SubjectTitle)
	if service == nil || service.store == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 ||
		command.TemplateID == 0 || command.SubjectType != "plan_unit" && command.SubjectType != "requirement" || command.SubjectID == 0 ||
		command.SubjectVersion == 0 || command.ProjectSetID == 0 || command.TeamID == 0 || command.SubjectTitle == "" ||
		command.ApplyKey == "" || len(command.InputDigest) != 64 || len(command.SubjectSnapshot) == 0 || !json.Valid(command.SubjectSnapshot) {
		return SubjectStartResult{}, ErrSubjectStartInvalid
	}
	return service.store.StartSubject(ctx, command)
}

func SubjectStageStatus(subjectType string, node DefinitionNode, instanceStatus string) string {
	if instanceStatus == "completed" {
		if subjectType == "requirement" {
			return "done"
		}
		return "released"
	}
	if configured, _ := node.Config["subject_status"].(string); validSubjectStatus(subjectType, configured) {
		return configured
	}
	name := strings.ToLower(strings.TrimSpace(node.Name))
	if subjectType == "requirement" {
		switch {
		case strings.Contains(name, "上线"), strings.Contains(name, "发布"), strings.Contains(name, "归档"):
			return "done"
		case strings.Contains(name, "测试"), strings.Contains(name, "验收"), strings.Contains(name, "提测"):
			return "acceptance"
		case strings.Contains(name, "研发"), strings.Contains(name, "开发"), strings.Contains(name, "任务拆解"):
			return "development"
		case strings.Contains(name, "评审"), strings.Contains(name, "审批"), strings.Contains(name, "补充"), strings.Contains(name, "设计"):
			return "grooming"
		case strings.Contains(name, "录入"), strings.Contains(name, "立项"), strings.Contains(name, "启动"):
			return "backlog"
		default:
			return "backlog"
		}
	}
	switch {
	case strings.Contains(name, "上线"), strings.Contains(name, "发布"):
		return "released"
	case strings.Contains(name, "测试"), strings.Contains(name, "验收"), strings.Contains(name, "提测"):
		return "acceptance"
	case strings.Contains(name, "研发"), strings.Contains(name, "开发"), strings.Contains(name, "执行"):
		return "development"
	case strings.Contains(name, "评审"), strings.Contains(name, "审批"), strings.Contains(name, "设计"):
		return "grooming"
	case strings.Contains(name, "录入"), strings.Contains(name, "立项"), strings.Contains(name, "启动"):
		return "planning"
	case strings.Contains(name, "需求"):
		return "grooming"
	default:
		return "planning"
	}
}

func SubjectStatusForCurrent(subjectType, instanceStatus string, nodes []DefinitionNode) string {
	if instanceStatus == "completed" {
		return SubjectStageStatus(subjectType, DefinitionNode{}, instanceStatus)
	}
	candidates := make([]DefinitionNode, 0, len(nodes))
	for _, node := range nodes {
		if nodeDrivesSubjectStatus(node) {
			candidates = append(candidates, node)
		}
	}
	if len(candidates) == 0 {
		candidates = nodes
	}
	chosen := ""
	for _, node := range candidates {
		status := SubjectStageStatus(subjectType, node, instanceStatus)
		if subjectStatusRank(subjectType, status) >= subjectStatusRank(subjectType, chosen) {
			chosen = status
		}
	}
	return chosen
}

func nodeDrivesSubjectStatus(node DefinitionNode) bool {
	switch node.Type {
	case "start", "end", "parallel_split", "parallel_join", "condition", "timer":
		return false
	}
	return !strings.HasPrefix(node.Type, "ai_")
}

func subjectStatusRank(subjectType, status string) int {
	order := []string{"planning", "grooming", "development", "acceptance", "released", "archived"}
	if subjectType == "requirement" {
		order = []string{"backlog", "grooming", "development", "acceptance", "done"}
	}
	for index, candidate := range order {
		if candidate == status {
			return index + 1
		}
	}
	return 0
}

func validSubjectStatus(subjectType, status string) bool {
	status = strings.TrimSpace(status)
	allowed := map[string]bool{"planning": true, "grooming": true, "development": true, "acceptance": true, "released": true, "archived": true}
	if subjectType == "requirement" {
		allowed = map[string]bool{"backlog": true, "grooming": true, "development": true, "acceptance": true, "done": true}
	}
	return status != "" && allowed[status]
}
