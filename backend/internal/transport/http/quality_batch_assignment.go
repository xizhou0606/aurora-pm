package httpapi

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"gorm.io/gorm"
)

type qualityBugBatchAssignmentBody struct {
	IDs []uint `json:"ids"`
	qualityBugBody
}

func (s *Server) batchAssignQualityBugs(c *gin.Context) {
	var body qualityBugBatchAssignmentBody
	if decodeStrictTaskNativeJSON(c, &body) != nil {
		bad(c, "invalid body")
		return
	}
	ids := uniqueTaskIDs(body.IDs)
	if len(ids) == 0 {
		bad(c, "ids required")
		return
	}
	if len(ids) > maxQualityBugBatchSize {
		bad(c, "too many ids")
		return
	}
	if !qualityBodyAssignmentOnly(body.qualityBugBody) {
		bad(c, "assignee is required and only assignment fields are allowed")
		return
	}
	if err := s.validateActivePMUserReferences(c, qualityBodyUserFactIDs(body.qualityBugBody, true)); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	updates, err := s.qualityBugUpdates(body.qualityBugBody)
	if err != nil {
		bad(c, err.Error())
		return
	}

	workspace, _ := currentWorkspace(c)
	var rows []qualitygorm.QualityBug
	query := s.db.Where("workspace_id = ? AND id IN ?", workspace.ID, ids)
	query = applyWorkspaceHierarchyScope(query, "quality_bugs", workspace.ID)
	if err := query.Find(&rows).Error; err != nil {
		dbFail(c)
		return
	}
	byID := make(map[uint]qualitygorm.QualityBug, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	ordered := make([]qualitygorm.QualityBug, 0, len(ids))
	for _, id := range ids {
		row, exists := byID[id]
		if !exists || !s.userCanAccessProject(c, workspace.ID, row.ProjectSetID) {
			failCode(c, http.StatusNotFound, apiErrorQualityBugBatchNotFound, "quality bug not found")
			return
		}
		ordered = append(ordered, row)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })

	actor, _ := currentActor(c)
	actorID := factUserID(actor.User)
	notificationContexts := make(map[uint]workspaceContext, len(ordered))
	notificationUserIDs := qualityBodyUserFactIDs(body.qualityBugBody, true)
	for _, bug := range ordered {
		notificationContexts[bug.ID] = s.loadWorkspaceContext(
			bug.WorkspaceID,
			workspaceContextReferences{}.withQualityBug(bug).withUserIDs(notificationUserIDs...),
		)
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		projectIDs := make([]uint, 0, len(ordered))
		for _, row := range ordered {
			projectIDs = append(projectIDs, row.ProjectSetID)
		}
		lockedProjects, lockErr := lockProjectSetsTx(tx, workspace.ID, projectIDs)
		if lockErr != nil {
			return lockErr
		}
		for _, before := range ordered {
			if _, exists := lockedProjects[before.ProjectSetID]; !exists {
				return gorm.ErrRecordNotFound
			}
			bug, lockErr := lockProjectQualityBugTx(tx, workspace.ID, before.ID, before.ProjectSetID)
			if lockErr != nil {
				return lockErr
			}
			before = bug
			workflowPayload, normalizeErr := s.normalizeQualityWorkflowBody(bug, &body.qualityBugBody)
			if normalizeErr != nil {
				return normalizeErr
			}
			if len(updates) > 0 {
				result := tx.Model(&qualitygorm.QualityBug{}).
					Where("id = ? AND workspace_id = ?", bug.ID, workspace.ID).
					Updates(updates)
				if result.Error != nil {
					return result.Error
				}
			}
			if err := tx.Where("id = ? AND workspace_id = ?", bug.ID, workspace.ID).First(&bug).Error; err != nil {
				return err
			}
			activitySummary, err := recordQualityPatchActivity(tx, before, bug, actorID, workflowPayload)
			if err != nil {
				return err
			}
			if err := s.recordMutationWithDetail(
				tx, c, bug.ProjectSetID, "quality_bug", bug.ID, "updated", bug.Title,
				activitySummary, updates,
			); err != nil {
				return err
			}
			if err := s.recordQualityPatchNotification(
				tx, c, before, bug, workflowPayload, notificationContexts[bug.ID],
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if respondQualityNoteLengthError(c, err) {
			return
		}
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"affected_ids": ids, "affected_count": len(ids), "atomic": true,
	})
}
