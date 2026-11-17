package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"gorm.io/gorm"
)

const maxQualityBugBatchSize = 100

type qualityBugBatchCreateBody struct {
	Bugs []qualityBugBatchCreateItem `json:"bugs"`
}

type qualityBugBatchCreateItem struct {
	ClientID string `json:"client_id"`
	Body     qualityBugBody
}

func (item *qualityBugBatchCreateItem) UnmarshalJSON(raw []byte) error {
	type metadata struct {
		ClientID string `json:"client_id"`
	}
	var meta metadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return err
	}
	var body qualityBugBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return err
	}
	item.ClientID = meta.ClientID
	item.Body = body
	return nil
}

type qualityBugBatchValidationField struct {
	ClientID string `json:"client_id,omitempty"`
	Field    string `json:"field"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type preparedQualityBugBatchItem struct {
	ClientID string
	Create   preparedQualityBugCreate
}

func (s *Server) batchCreateQualityBugs(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body qualityBugBatchCreateBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	prepared, fields := s.prepareQualityBugBatch(c, workspace, body)
	if len(fields) > 0 {
		if fields[0].Code == validationPMUserRegistryUnavailable {
			failCode(c, http.StatusInternalServerError, apiErrorPMUserRegistryUnavailable,
				"PM user registry unavailable")
			return
		}
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
			"code":  apiErrorQualityBugBatchValidationFailed,
			"error": "batch quality bug validation failed", "fields": fields,
		})
		return
	}
	authActor, _ := currentActor(c)
	createdRows := make([]gin.H, 0, len(prepared))
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for index := range prepared {
			item := &prepared[index]
			if err := s.createPreparedQualityBug(tx, c, authActor, &item.Create); err != nil {
				return err
			}
			createdRows = append(createdRows, gin.H{
				"client_id": item.ClientID, "id": item.Create.Bug.ID,
				"bug_key": item.Create.Bug.BugKey,
			})
		}
		return nil
	})
	if err != nil {
		dbFail(c)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"created": createdRows, "created_count": len(createdRows), "atomic": true,
	}})
}

func (s *Server) prepareQualityBugBatch(
	c *gin.Context,
	workspace identitygorm.Workspace,
	body qualityBugBatchCreateBody,
) ([]preparedQualityBugBatchItem, []qualityBugBatchValidationField) {
	if len(body.Bugs) == 0 {
		return nil, []qualityBugBatchValidationField{{
			Field: "bugs", Code: validationRequired, Message: "bugs required",
		}}
	}
	if len(body.Bugs) > maxQualityBugBatchSize {
		return nil, []qualityBugBatchValidationField{{
			Field: "bugs", Code: validationMaximumItems, Message: "too many quality bugs",
		}}
	}
	authActor, _ := currentActor(c)
	seenClientIDs := make(map[string]struct{}, len(body.Bugs))
	prepared := make([]preparedQualityBugBatchItem, 0, len(body.Bugs))
	for _, item := range body.Bugs {
		clientID, validation := reserveQualityBatchClientID(seenClientIDs, item.ClientID)
		if validation != nil {
			return nil, []qualityBugBatchValidationField{*validation}
		}
		create, err := s.prepareQualityBugCreate(c, workspace, authActor, item.Body)
		if err != nil {
			return nil, []qualityBugBatchValidationField{
				qualityBugBatchValidation(clientID, err),
			}
		}
		prepared = append(prepared, preparedQualityBugBatchItem{ClientID: clientID, Create: create})
	}
	return prepared, nil
}

func reserveQualityBatchClientID(
	seen map[string]struct{},
	raw string,
) (string, *qualityBugBatchValidationField) {
	clientID, err := reserveTaskBatchClientID(seen, raw)
	if err == nil {
		return clientID, nil
	}
	field := qualityBugBatchValidationField{
		ClientID: strings.TrimSpace(raw), Field: "client_id",
		Code: validationInvalid, Message: err.Error(),
	}
	var validation *taskBatchValidationError
	if errors.As(err, &validation) && len(validation.Fields) > 0 {
		field.ClientID = validation.Fields[0].ClientID
		field.Code = validation.Fields[0].Code
		field.Message = validation.Fields[0].Message
	}
	return "", &field
}

func qualityBugBatchValidation(clientID string, err error) qualityBugBatchValidationField {
	field := qualityBugBatchValidationField{
		ClientID: clientID, Field: "bugs", Code: validationInvalid, Message: err.Error(),
	}
	var validation *qualityBugCreateError
	if !errors.As(err, &validation) {
		return field
	}
	if validation.PlatformUser {
		field.Field = "assignee_user_id"
		field.Code = validationInvalidUser
		if errors.Is(validation.Cause, errPMUserRegistryUnavailable) {
			field.Code = validationPMUserRegistryUnavailable
		}
		return field
	}
	if validation.Status == http.StatusNotFound {
		field.Field = "requirement_id"
		field.Code = validationInvalidScope
		return field
	}
	for _, candidate := range []string{
		"requirement_id", "title", "status", "priority", "severity_key", "defect_domain",
		"resolution_reason", "due_at", "due_date", "assignee_user_id",
		"reviewer_user_id", "source_execution_id", "source_test_id",
	} {
		if strings.Contains(validation.Message, candidate) {
			field.Field = candidate
			break
		}
	}
	return field
}

func (s *Server) batchDeleteQualityBugs(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
	}
	if c.ShouldBindJSON(&body) != nil {
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
		if !exists {
			failCode(c, http.StatusNotFound, apiErrorQualityBugBatchNotFound,
				"quality bug not found")
			return
		}
		if !s.userCanAccessProject(c, workspace.ID, row.ProjectSetID) {
			failCode(c, http.StatusNotFound, apiErrorQualityBugBatchNotFound,
				"quality bug not found")
			return
		}
		ordered = append(ordered, row)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"workspace_id = ? AND quality_bug_id IN ?", workspace.ID, ids,
		).Delete(&qualitygorm.QualityBugComment{}).Error; err != nil {
			return err
		}
		if err := tx.Where(
			"workspace_id = ? AND quality_bug_id IN ?", workspace.ID, ids,
		).Delete(&qualitygorm.QualityBugActivity{}).Error; err != nil {
			return err
		}
		for _, bug := range ordered {
			if err := s.recordMutation(
				tx, c, bug.ProjectSetID, "quality_bug", bug.ID, "deleted", bug.Title, bug,
			); err != nil {
				return err
			}
		}
		result := tx.Where("workspace_id = ? AND id IN ?", workspace.ID, ids).
			Delete(&qualitygorm.QualityBug{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(ids)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"deleted_ids": ids, "affected_ids": ids,
		"deleted_count": len(ids), "affected_count": len(ids), "atomic": true,
	})
}
