package httpapi

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

const workflowNodeOpinionBodyLimit = 20_000

type workflowNodeOpinionBody struct {
	Body          string `json:"body"`
	InviteUserIDs []uint `json:"invite_user_ids"`
}

func (s *Server) listWorkflowNodeOpinions(c *gin.Context) {
	instance, node, okNode := s.workflowPreReviewOpinionNode(c)
	if !okNode {
		return
	}
	var rows []workflowgorm.WorkflowNodeOpinion
	if s.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ?", instance.WorkspaceID, instance.ID, node.ID).
		Order("id DESC").Find(&rows).Error != nil {
		dbFail(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": s.workflowNodeOpinionDTOs(c, rows), "total": len(rows)})
}

func (s *Server) createWorkflowNodeOpinion(c *gin.Context) {
	instance, node, okNode := s.workflowPreReviewOpinionNode(c)
	if !okNode {
		return
	}
	bodyText, inviteIDs, okBody := parseWorkflowNodeOpinionRequest(c)
	if !okBody {
		return
	}
	projectID := s.workflowSubjectProjectID(s.db, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
	if bodyText == "" {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			if projectID != 0 {
				if _, err := lockProjectSetTx(tx, instance.WorkspaceID, projectID); err != nil {
					return err
				}
			}
			return s.recordWorkflowPreReviewInvitedNotification(tx, c, instance, node, inviteIDs)
		})
		if err != nil {
			if respondProjectChildScopeConflict(c, err) {
				return
			}
			dbFail(c)
			return
		}
		ok(c, gin.H{"invited_user_ids": inviteIDs})
		return
	}
	now := time.Now().UTC()
	opinion := workflowgorm.WorkflowNodeOpinion{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID,
		AuthorUserID: currentFactUserID(c), Body: bodyText, CreatedAt: now, UpdatedAt: now,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if projectID != 0 {
			if _, err := lockProjectSetTx(tx, instance.WorkspaceID, projectID); err != nil {
				return err
			}
		}
		if err := tx.Create(&opinion).Error; err != nil {
			return err
		}
		if err := s.recordWorkflowPreReviewInvitedNotification(tx, c, instance, node, inviteIDs); err != nil {
			return err
		}
		if projectID == 0 {
			return nil
		}
		return s.recordMutation(
			tx, c, projectID, "workflow_node_opinion", opinion.ID, "created", node.Name, opinion,
		)
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		dbFail(c)
		return
	}
	created(c, s.workflowNodeOpinionDTO(c, opinion))
}

func (s *Server) updateWorkflowNodeOpinion(c *gin.Context) {
	instance, node, okNode := s.workflowPreReviewOpinionNode(c)
	if !okNode {
		return
	}
	opinionID, valid := paramID(c, "opinionId")
	if !valid {
		return
	}
	bodyText, okBody := parseWorkflowNodeOpinionBody(c)
	if !okBody {
		return
	}
	var opinion workflowgorm.WorkflowNodeOpinion
	if s.db.Where(
		"id = ? AND workspace_id = ? AND instance_id = ? AND node_id = ?",
		opinionID, instance.WorkspaceID, instance.ID, node.ID,
	).First(&opinion).Error != nil {
		fail(c, http.StatusNotFound, "workflow node opinion not found")
		return
	}
	projectID := s.workflowSubjectProjectID(s.db, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
	now := time.Now().UTC()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if projectID != 0 {
			if _, err := lockProjectSetTx(tx, instance.WorkspaceID, projectID); err != nil {
				return err
			}
		}
		result := tx.Model(&workflowgorm.WorkflowNodeOpinion{}).
			Where("id = ? AND workspace_id = ? AND instance_id = ? AND node_id = ?", opinion.ID, opinion.WorkspaceID, opinion.InstanceID, opinion.NodeID).
			Updates(map[string]any{"body": bodyText, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		opinion.Body = bodyText
		opinion.UpdatedAt = now
		if projectID == 0 {
			return nil
		}
		return s.recordMutation(
			tx, c, projectID, "workflow_node_opinion", opinion.ID, "updated", node.Name, opinion,
		)
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		if err == gorm.ErrRecordNotFound {
			fail(c, http.StatusConflict, "workflow node opinion changed")
			return
		}
		dbFail(c)
		return
	}
	ok(c, s.workflowNodeOpinionDTO(c, opinion))
}

func parseWorkflowNodeOpinionBody(c *gin.Context) (string, bool) {
	text, _, ok := parseWorkflowNodeOpinionRequest(c)
	if !ok || text == "" {
		if ok && text == "" {
			bad(c, "body required")
		}
		return "", false
	}
	return text, true
}

func parseWorkflowNodeOpinionRequest(c *gin.Context) (string, []uint, bool) {
	var body workflowNodeOpinionBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return "", nil, false
	}
	invites := uniquePositiveIDs(body.InviteUserIDs)
	text := strings.TrimSpace(body.Body)
	if text == "" && len(invites) == 0 {
		bad(c, "body required")
		return "", nil, false
	}
	if text == "" {
		return "", invites, true
	}
	if strings.Contains(strings.ToLower(text), "<script") {
		bad(c, "body contains unsupported markup")
		return "", nil, false
	}
	if utf8.RuneCountInString(text) > workflowNodeOpinionBodyLimit {
		bad(c, "body is too long")
		return "", nil, false
	}
	return text, invites, true
}

func (s *Server) workflowPreReviewOpinionNode(c *gin.Context) (workflowgorm.WorkflowInstance, workflowNode, bool) {
	instance, ok := s.workflowInstanceByID(c)
	if !ok {
		return workflowgorm.WorkflowInstance{}, workflowNode{}, false
	}
	var version workflowgorm.WorkflowTemplateVersion
	if s.db.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).First(&version).Error != nil {
		dbFail(c)
		return workflowgorm.WorkflowInstance{}, workflowNode{}, false
	}
	definition, issues := compileWorkflowDefinition(version.Definition)
	nodeID := strings.TrimSpace(c.Param("nodeId"))
	node, found := workflowNodeByID(definition, nodeID)
	if len(issues) > 0 || !found || !workflowNodeSupportsPreReviewOpinions(node) {
		fail(c, http.StatusNotFound, "pre-review approval node not found")
		return workflowgorm.WorkflowInstance{}, workflowNode{}, false
	}
	return instance, node, true
}

func workflowNodeSupportsPreReviewOpinions(node workflowNode) bool {
	if node.Type != "approval" {
		return false
	}
	if strings.Contains(node.Name, "预审") {
		return true
	}
	marker := strings.ToLower(strings.TrimSpace(node.ID + " " + node.Name))
	for _, token := range []string{"pre-review", "pre_review", "prereview", "preapproval", "pre-approval", "pre_approval"} {
		if strings.Contains(marker, token) {
			return true
		}
	}
	return false
}

func (s *Server) workflowNodeOpinionDTOs(c *gin.Context, rows []workflowgorm.WorkflowNodeOpinion) []gin.H {
	workspace, _ := currentWorkspace(c)
	refs := workspaceContextReferences{}
	for _, row := range rows {
		refs = refs.withUserID(row.AuthorUserID)
	}
	ctx := s.loadWorkspaceContext(workspace.ID, refs)
	result := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		result = append(result, workflowNodeOpinionDTOWithContext(row, ctx))
	}
	return result
}

func (s *Server) workflowNodeOpinionDTO(c *gin.Context, row workflowgorm.WorkflowNodeOpinion) gin.H {
	workspace, _ := currentWorkspace(c)
	return workflowNodeOpinionDTOWithContext(row, s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withUserID(row.AuthorUserID),
	))
}

func workflowNodeOpinionDTOWithContext(row workflowgorm.WorkflowNodeOpinion, ctx workspaceContext) gin.H {
	authorID := row.AuthorUserID
	result := gin.H{
		"id": row.ID, "workspace_id": row.WorkspaceID, "instance_id": row.InstanceID, "node_id": row.NodeID,
		"author_user_id": row.AuthorUserID, "body": row.Body,
		"created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
		"edited": row.UpdatedAt.After(row.CreatedAt),
	}
	if user, ok := ctx.Users[row.AuthorUserID]; ok {
		name := strings.TrimSpace(user.DisplayName)
		if name == "" {
			name = strings.TrimSpace(user.Username)
		}
		result["author_display_name"] = name
		result["author_username"] = user.Username
		result["author_avatar_url"] = user.AvatarURL
	} else {
		result["author_display_name"] = retiredUserDisplayName(row.AuthorUserID)
		result["author_username"] = ""
		result["author_avatar_url"] = userAvatarURL(ctx, &authorID)
	}
	return result
}

func uniquePositiveIDs(values []uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
