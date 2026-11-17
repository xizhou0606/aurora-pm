package httpapi

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

const (
	biTeamMemberItemRequirement = "requirement"
	biTeamMemberItemTask        = "task"
	biTeamMemberItemBug         = "bug"
)

type biTeamMemberItemDTO struct {
	ID                uint       `json:"id"`
	Key               string     `json:"key,omitempty"`
	Title             string     `json:"title"`
	Status            string     `json:"status"`
	Priority          string     `json:"priority,omitempty"`
	Severity          string     `json:"severity,omitempty"`
	ProjectID         uint       `json:"project_id,omitempty"`
	RequirementID     uint       `json:"requirement_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	DueAt             *time.Time `json:"due_at,omitempty"`
	EffectiveDueAt    *time.Time `json:"effective_due_at,omitempty"`
	DueAtSource       string     `json:"due_at_source,omitempty"`
	DueAtSourceTaskID uint       `json:"due_at_source_task_id,omitempty"`
	DeliveryAt        *time.Time `json:"delivery_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	Overdue           *bool      `json:"overdue,omitempty"`
}

func (s *Server) listBITeamMemberItems(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	scope, err := s.parseWorkspaceScope(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	dimension, dimensionKey, err := parseBITeamDimension(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	userIDValue, err := strconv.ParseUint(strings.TrimSpace(c.Query("user_id")), 10, 64)
	if err != nil || userIDValue == 0 {
		bad(c, "user_id must be a positive id")
		return
	}
	role := normalizeStatus(c.Query("role"))
	itemType := normalizeStatus(c.Query("item_type"))
	if err := validateBITeamMemberItemRequest(role, itemType); err != nil {
		bad(c, err.Error())
		return
	}

	items, err := s.loadBITeamMemberItems(
		c,
		workspace.ID,
		scope,
		uint(userIDValue),
		role,
		itemType,
		dimension,
		dimensionKey,
		biLocation(c),
	)
	if err != nil {
		dbFail(c)
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	ok(c, gin.H{
		"role": role, "item_type": itemType, "user_id": uint(userIDValue),
		"dimension": dimension, "dimension_key": dimensionKey,
		"total": len(items), "items": items,
	})
}

func validateBITeamMemberItemRequest(role, itemType string) error {
	valid := role == "product" && itemType == biTeamMemberItemRequirement ||
		role == "testing" && itemType == biTeamMemberItemBug ||
		role == "development" && (itemType == biTeamMemberItemTask || itemType == biTeamMemberItemBug)
	if !valid {
		return fmt.Errorf("role and item_type are not a supported BI member drilldown")
	}
	return nil
}

func (s *Server) loadBITeamMemberItems(
	c *gin.Context,
	workspaceID uint,
	scope workspaceScope,
	userID uint,
	role string,
	itemType string,
	dimension string,
	dimensionKey string,
	location *time.Location,
) ([]biTeamMemberItemDTO, error) {
	switch itemType {
	case biTeamMemberItemRequirement:
		rows, err := s.loadBIScopedRequirements(c, workspaceID, scope)
		if err != nil {
			return nil, err
		}
		result := make([]biTeamMemberItemDTO, 0)
		for _, row := range rows {
			if role != "product" || row.ProductOwnerUserID == nil || *row.ProductOwnerUserID != userID ||
				!biTeamRecordMatches(row.PlanUnitID, row.CreatedAt, dimension, dimensionKey, location) {
				continue
			}
			result = append(result, biTeamRequirementItem(row))
		}
		return result, nil
	case biTeamMemberItemTask:
		query := applyWorkspaceScope(
			pmTaskQuery(s.db).Where("workspace_id = ?", workspaceID), scope,
		)
		query = applyWorkspaceHierarchyScope(query, "work_tasks", workspaceID)
		s.applyProjectDataFilter(c, query, "work_tasks", "project_set_id")
		var rows []taskgorm.WorkTask
		if err := query.Find(&rows).Error; err != nil {
			return nil, err
		}
		activities, err := s.loadBITaskActivity(workspaceID, rows)
		if err != nil {
			return nil, err
		}
		taskSnapshots := legacyBITaskSnapshots(rows)
		dueFacts := biTeamTaskDueFacts(taskSnapshots)
		taskFacts := biTeamTaskActivityFacts(taskSnapshots, legacyBITaskActivitySnapshots(activities))
		now := time.Now().UTC()
		result := make([]biTeamMemberItemDTO, 0)
		for _, row := range rows {
			if role != "development" || row.AssigneeID == nil || *row.AssigneeID != userID ||
				!biTeamRecordMatches(row.ProjectID, row.CreatedAt, dimension, dimensionKey, location) {
				continue
			}
			result = append(result, biTeamTaskItem(row, dueFacts[row.ID], taskFacts[row.ID], now))
		}
		return result, nil
	case biTeamMemberItemBug:
		query := applyWorkspaceScope(
			s.db.Model(&qualitygorm.QualityBug{}).Where("workspace_id = ?", workspaceID), scope,
		)
		query = applyWorkspaceHierarchyScope(query, "quality_bugs", workspaceID)
		s.applyProjectDataFilter(c, query, "quality_bugs", "project_set_id")
		var rows []qualitygorm.QualityBug
		if err := query.Find(&rows).Error; err != nil {
			return nil, err
		}
		result := make([]biTeamMemberItemDTO, 0)
		for _, row := range rows {
			ownerMatches := role == "development" && row.AssigneeUserID != nil && *row.AssigneeUserID == userID
			reporterMatches := role == "testing" && row.ReporterUserID != nil && *row.ReporterUserID == userID
			if (!ownerMatches && !reporterMatches) || !biEffectiveQualityBug(legacyBIBugSnapshot(row)) ||
				!biTeamRecordMatches(row.ProjectID, row.CreatedAt, dimension, dimensionKey, location) {
				continue
			}
			result = append(result, biTeamBugItem(row))
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported BI member item type")
	}
}

func legacyBITaskSnapshots(rows []taskgorm.WorkTask) []task.Snapshot {
	result := make([]task.Snapshot, 0, len(rows))
	for _, row := range rows {
		result = append(result, task.Snapshot{ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID,
			ProjectSetID: row.ProjectSetID, PlanUnitID: row.ProjectID, RequirementID: row.RequirementID,
			Kind: row.Kind, TaskKey: row.TaskKey, Title: row.Title, Status: row.Status, Priority: row.Priority,
			AssigneeID: row.AssigneeID, ReporterUserID: row.ReporterUserID, ParentID: row.ParentID,
			EstimatedMinutes: row.EstimatedMinutes, StartsAt: row.StartsAt, DueAt: row.DueAt,
			CompletedAt: row.CompletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	return result
}

func legacyBITaskActivitySnapshots(rows []taskgorm.WorkTaskActivity) []task.ActivitySnapshot {
	result := make([]task.ActivitySnapshot, 0, len(rows))
	for _, row := range rows {
		result = append(result, task.ActivitySnapshot{ID: row.ID, WorkspaceID: row.WorkspaceID,
			WorkTaskID: row.WorkTaskID, UserID: row.UserID, Action: row.Action, Detail: row.Detail,
			FieldKey: row.FieldKey, OldValue: row.OldValue, NewValue: row.NewValue, Summary: row.Summary, CreatedAt: row.CreatedAt})
	}
	return result
}

func legacyBIBugSnapshot(row qualitygorm.QualityBug) quality.BugSnapshot {
	return quality.BugSnapshot{ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID,
		ProjectSetID: row.ProjectSetID, PlanUnitID: row.ProjectID, RequirementID: row.RequirementID,
		BugKey: row.BugKey, Title: row.Title, Status: row.Status, Priority: row.Priority,
		SeverityKey: row.SeverityKey, DueAt: row.DueAt, ResolutionReason: row.ResolutionReason,
		AssigneeUserID: row.AssigneeUserID, ReporterUserID: row.ReporterUserID, ReviewerUserID: row.ReviewerUserID,
		ResolvedAt: row.ResolvedAt, ReviewedAt: row.ReviewedAt, CompletedAt: row.CompletedAt,
		CanceledAt: row.CanceledAt, StatusChangedAt: row.StatusChangedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func legacyBIBugActivitySnapshots(rows []qualitygorm.QualityBugActivity) []quality.BugActivitySnapshot {
	result := make([]quality.BugActivitySnapshot, 0, len(rows))
	for _, row := range rows {
		result = append(result, quality.BugActivitySnapshot{ID: row.ID, WorkspaceID: row.WorkspaceID,
			QualityBugID: row.QualityBugID, UserID: row.UserID, Action: row.Action, FieldKey: row.FieldKey,
			OldValue: row.OldValue, NewValue: row.NewValue, Summary: row.Summary, CreatedAt: row.CreatedAt})
	}
	return result
}

func biTeamRequirementItem(row requirementgorm.Requirement) biTeamMemberItemDTO {
	return biTeamMemberItemDTO{
		ID: row.ID, Title: row.Title, Status: row.Status, Priority: row.Priority,
		ProjectID: row.PlanUnitID, RequirementID: row.ID, CreatedAt: row.CreatedAt,
	}
}

func biTeamTaskItem(
	row taskgorm.WorkTask,
	dueFact biTeamTaskDueFact,
	taskFact biTeamTaskFacts,
	now time.Time,
) biTeamMemberItemDTO {
	overdue := !task.IsCancelledStatus(row.Status) && biTeamTaskOverdue(dueFact.dueAt, taskFact.deliveryAt, now)
	return biTeamMemberItemDTO{
		ID: row.ID, Key: row.TaskKey, Title: row.Title, Status: row.Status, Priority: row.Priority,
		ProjectID: row.ProjectID, RequirementID: row.RequirementID, CreatedAt: row.CreatedAt,
		DueAt: row.DueAt, EffectiveDueAt: dueFact.dueAt, DueAtSource: dueFact.source,
		DueAtSourceTaskID: dueFact.sourceTaskID, DeliveryAt: taskFact.deliveryAt,
		CompletedAt: row.CompletedAt, Overdue: &overdue,
	}
}

func biTeamBugItem(row qualitygorm.QualityBug) biTeamMemberItemDTO {
	return biTeamMemberItemDTO{
		ID: row.ID, Key: row.BugKey, Title: row.Title, Status: row.Status,
		Priority: row.Priority, Severity: row.SeverityKey, ProjectID: row.ProjectID,
		RequirementID: row.RequirementID, CreatedAt: row.CreatedAt, DueAt: row.DueAt,
		CompletedAt: row.CompletedAt,
	}
}
