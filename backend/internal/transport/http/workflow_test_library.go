package httpapi

import (
	"strings"

	"github.com/gin-gonic/gin"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func workflowTestLibraryItemDTO(
	artifact workflowgorm.WorkflowNodeArtifact,
	instance workflowgorm.WorkflowInstance,
	requirements map[uint]requirementgorm.Requirement,
	planUnits map[uint]projectgorm.PlanUnit,
	projects map[uint]projectgorm.ProjectSet,
	uploaders map[uint]identitygorm.User,
) gin.H {
	kind := "test_case"
	if artifact.Kind == workflowTestReportArtifactKind {
		kind = "test_report"
	}
	dto := workflowNodeArtifactDTO(artifact)
	item := gin.H{
		"id": artifact.ID, "kind": kind, "name": artifact.Name, "content_type": artifact.ContentType,
		"size": artifact.Size, "url": dto["url"], "source": dto["source"], "created_at": artifact.CreatedAt,
		"instance_id": artifact.InstanceID, "node_id": artifact.NodeID,
		"subject_type": instance.SubjectType, "subject_id": instance.SubjectID,
		"uploader_user_id": artifact.UploaderUserID,
	}
	if user, ok := uploaders[artifact.UploaderUserID]; ok {
		name := strings.TrimSpace(user.DisplayName)
		if name == "" {
			name = user.Username
		}
		item["uploader_name"] = name
		if avatar := strings.TrimSpace(user.AvatarURL); avatar != "" {
			item["uploader_avatar_url"] = avatar
		}
	}
	var projectID, unitID uint
	subjectTitle := ""
	if instance.SubjectType == "requirement" {
		if requirement, ok := requirements[instance.SubjectID]; ok {
			subjectTitle = requirement.Title
			projectID = requirement.ProjectSetID
			if requirement.ScheduledPlanUnitID != nil {
				unitID = *requirement.ScheduledPlanUnitID
			}
		}
	} else if unit, ok := planUnits[instance.SubjectID]; ok {
		subjectTitle = unit.Title
		projectID = unit.ProjectSetID
		unitID = unit.ID
	}
	item["subject_title"] = subjectTitle
	if projectID > 0 {
		item["project_id"] = projectID
		if project, ok := projects[projectID]; ok {
			item["project_name"] = project.Name
		}
	}
	if unitID > 0 {
		item["plan_unit_id"] = unitID
		if unit, ok := planUnits[unitID]; ok {
			item["plan_unit_title"] = unit.Title
		}
	}
	return item
}

func workflowTestLibraryItemMatches(item gin.H, query string) bool {
	needle := strings.ToLower(query)
	for _, key := range []string{"name", "subject_title", "project_name", "plan_unit_title", "uploader_name"} {
		value, _ := item[key].(string)
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

func uniqueWorkflowArtifactInstanceIDs(rows []workflowgorm.WorkflowNodeArtifact) []uint {
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.InstanceID)
	}
	return uniqueUintIDs(ids)
}

func workflowLibrarySubjectIDs(instances map[uint]workflowgorm.WorkflowInstance) ([]uint, []uint) {
	requirementIDs := make([]uint, 0)
	planUnitIDs := make([]uint, 0)
	for _, instance := range instances {
		if instance.SubjectType == "requirement" {
			requirementIDs = append(requirementIDs, instance.SubjectID)
			continue
		}
		planUnitIDs = append(planUnitIDs, instance.SubjectID)
	}
	return uniqueUintIDs(requirementIDs), uniqueUintIDs(planUnitIDs)
}

func uniqueUintIDs(values []uint) []uint {
	seen := make(map[uint]struct{}, len(values))
	out := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func uintFromAny(value any) uint {
	switch typed := value.(type) {
	case uint:
		return typed
	case int:
		if typed > 0 {
			return uint(typed)
		}
	}
	return 0
}
