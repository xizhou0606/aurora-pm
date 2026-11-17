package httpapi

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

const (
	requirementPoolNodeIdle      = "__idle__"
	requirementPoolNodeCompleted = "__completed__"
)

type requirementPoolSummary struct {
	Total       int64 `json:"total"`
	Unscheduled int64 `json:"unscheduled"`
	Delivery    int64 `json:"delivery"`
}

func (s *Server) requirementPoolQuery(
	c *gin.Context,
	workspaceID uint,
) (*gorm.DB, bool) {
	query := s.db.Model(&requirementgorm.Requirement{}).Where("workspace_id = ?", workspaceID)
	if !s.applyProjectDataFilter(c, query, "requirements", "project_new_id") {
		return nil, false
	}
	projectIDs, ok := parseRequirementPoolUintCSV(c.Query("project_new_id"))
	if !ok {
		bad(c, "project_new_id is invalid")
		return nil, false
	}
	planUnitIDs, unscheduled, ok := parseRequirementPoolPlanUnits(c.Query("version_task_new_id"))
	if !ok {
		bad(c, "version_task_new_id is invalid")
		return nil, false
	}
	query = applyRequirementPoolScopeFilter(query, projectIDs, planUnitIDs, unscheduled)
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		if !validRequirementStatus(status) {
			bad(c, "status is invalid")
			return nil, false
		}
		query = query.Where("status = ?", normalizeStatus(status))
	}
	if priority := strings.TrimSpace(c.Query("priority")); priority != "" {
		if !validPriority(priority) {
			bad(c, "priority is invalid")
			return nil, false
		}
		query = query.Where("priority = ?", normalizePriority(priority))
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		if id, err := strconv.ParseUint(search, 10, 64); err == nil && id > 0 {
			query = query.Where(
				"id = ? OR LOWER(title) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ?",
				id,
				pattern,
				pattern,
			)
		} else {
			query = query.Where(
				"LOWER(title) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ?",
				pattern,
				pattern,
			)
		}
	}
	nodeIDs, ok := parseRequirementPoolNodeIDs(c.Query("current_node_id"))
	if !ok {
		bad(c, "current_node_id is invalid")
		return nil, false
	}
	if len(nodeIDs) > 0 {
		filtered, ok := s.applyRequirementCurrentNodeFilter(query, workspaceID, nodeIDs)
		if !ok {
			bad(c, "current_node_id is invalid")
			return nil, false
		}
		query = filtered
	}
	for _, owner := range []struct {
		query  string
		scalar string
		list   string
	}{
		{"product_owner_user_id", "product_owner_user_id", "product_owner_user_ids"},
		{"design_owner_user_id", "design_owner_user_id", "design_owner_user_ids"},
		{"frontend_owner_user_id", "frontend_owner_user_id", "frontend_owner_user_ids"},
		{"backend_owner_user_id", "backend_owner_user_id", "backend_owner_user_ids"},
		{"tester_user_id", "", "tester_user_ids"},
	} {
		userIDs, ok := parseRequirementPoolUintCSV(c.Query(owner.query))
		if !ok {
			bad(c, owner.query+" is invalid")
			return nil, false
		}
		if len(userIDs) > 0 {
			query = applyRequirementOwnerFilter(query, owner.scalar, owner.list, userIDs)
		}
	}
	return query, true
}

func applyRequirementPoolScopeFilter(
	query *gorm.DB,
	projectIDs []uint,
	planUnitIDs []uint,
	unscheduled bool,
) *gorm.DB {
	if len(projectIDs) == 0 && len(planUnitIDs) == 0 && !unscheduled {
		return query
	}
	if len(projectIDs) > 0 && len(planUnitIDs) == 0 && !unscheduled {
		return query.Where("project_new_id IN ?", projectIDs)
	}
	scope := query.Session(&gorm.Session{NewDB: true})
	first := true
	add := func(sql string, args ...any) {
		if first {
			scope = scope.Where(sql, args...)
			first = false
			return
		}
		scope = scope.Or(sql, args...)
	}
	if len(projectIDs) > 0 {
		add("project_new_id IN ?", projectIDs)
	}
	if len(planUnitIDs) > 0 {
		add("version_task_new_id IN ?", planUnitIDs)
	}
	if unscheduled {
		add("version_task_new_id IS NULL")
	}
	return query.Where(scope)
}

func (s *Server) applyRequirementCurrentNodeFilter(
	query *gorm.DB,
	workspaceID uint,
	nodeIDs []string,
) (*gorm.DB, bool) {
	instances := s.db.Model(&workflowgorm.WorkflowInstance{}).
		Select("id").
		Where("workspace_id = ? AND subject_type = ?", workspaceID, "requirement")
	scope := query.Session(&gorm.Session{NewDB: true})
	first := true
	add := func(sql string, args ...any) {
		if first {
			scope = scope.Where(sql, args...)
			first = false
			return
		}
		scope = scope.Or(sql, args...)
	}
	for _, nodeID := range nodeIDs {
		switch nodeID {
		case requirementPoolNodeIdle:
			add("workflow_instance_id IS NULL")
		case requirementPoolNodeCompleted:
			add("workflow_instance_id IN (?)", instances.Session(&gorm.Session{}).Where("status = ?", "completed"))
		default:
			if !validRequirementPoolNodeID(nodeID) {
				return nil, false
			}
			add(
				"workflow_instance_id IN (?)",
				instances.Session(&gorm.Session{}).Where(
					"current_node_ids LIKE ? ESCAPE ?",
					`%"`+escapeRequirementPoolLike(nodeID)+`"%`,
					`\`,
				),
			)
		}
	}
	return query.Where(scope), true
}

func applyRequirementOwnerFilter(query *gorm.DB, scalarColumn, listColumn string, userIDs []uint) *gorm.DB {
	parts := make([]string, 0, len(userIDs))
	args := make([]any, 0, len(userIDs)*5)
	for _, userID := range userIDs {
		token := strconv.FormatUint(uint64(userID), 10)
		listValue := "CAST(" + listColumn + " AS TEXT)"
		listClause := listValue + " = ? OR " + listValue + " LIKE ? OR " + listValue + " LIKE ? OR " + listValue + " LIKE ?"
		listArgs := []any{"[" + token + "]", "[" + token + ",%", "%," + token + ",%", "%," + token + "]"}
		if scalarColumn != "" {
			parts = append(parts, "("+scalarColumn+" = ? OR "+listClause+")")
			args = append(args, userID)
		} else {
			parts = append(parts, "("+listClause+")")
		}
		args = append(args, listArgs...)
	}
	return query.Where(strings.Join(parts, " OR "), args...)
}

func loadRequirementPoolSummary(
	query *gorm.DB,
	total int64,
) (requirementPoolSummary, error) {
	summary := requirementPoolSummary{Total: total}
	err := query.Session(&gorm.Session{}).
		Select(`
			COALESCE(SUM(CASE WHEN version_task_new_id IS NULL THEN 1 ELSE 0 END), 0) AS unscheduled,
			COALESCE(SUM(CASE WHEN status IN ? THEN 1 ELSE 0 END), 0) AS delivery
		`, []string{"development", "acceptance"}).
		Scan(&summary).Error
	summary.Total = total
	return summary, err
}

func splitRequirementPoolCSV(raw string) []string {
	values := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func parseRequirementPoolUintCSV(raw string) ([]uint, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	parts := splitRequirementPoolCSV(raw)
	if len(parts) == 0 {
		return nil, false
	}
	seen := map[uint]struct{}{}
	ids := make([]uint, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil || value == 0 {
			return nil, false
		}
		id := uint(value)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, true
}

func parseRequirementPoolPlanUnits(raw string) ([]uint, bool, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, true
	}
	parts := splitRequirementPoolCSV(raw)
	if len(parts) == 0 {
		return nil, false, false
	}
	seen := map[uint]struct{}{}
	ids := make([]uint, 0, len(parts))
	unscheduled := false
	for _, part := range parts {
		if part == "none" {
			unscheduled = true
			continue
		}
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil || value == 0 {
			return nil, false, false
		}
		id := uint(value)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 && !unscheduled {
		return nil, false, false
	}
	return ids, unscheduled, true
}

func parseRequirementPoolNodeIDs(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	parts := splitRequirementPoolCSV(raw)
	if len(parts) == 0 {
		return nil, false
	}
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		ids = append(ids, part)
	}
	return ids, true
}

func validRequirementPoolNodeID(nodeID string) bool {
	return nodeID != "" && len(nodeID) <= 100 && !strings.ContainsAny(nodeID, `"\%`)
}

func escapeRequirementPoolLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
