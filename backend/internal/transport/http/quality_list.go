package httpapi

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"gorm.io/gorm"
)

func (s *Server) listQualityBugs(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	scope, err := s.parseWorkspaceScope(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	query := applyWorkspaceScope(
		s.db.Model(&qualitygorm.QualityBug{}).Where("workspace_id = ?", workspace.ID), scope,
	)
	query = applyWorkspaceHierarchyScope(query, "quality_bugs", workspace.ID)
	if !s.applyProjectDataFilter(c, query, "quality_bugs", "project_set_id") {
		query = query.Where("1 = 0")
	}
	query, err = applyQualityBugFilters(c, query, true)
	if err != nil {
		bad(c, err.Error())
		return
	}
	var total int64
	if query.Session(&gorm.Session{}).Count(&total).Error != nil {
		dbFail(c)
		return
	}
	limit, offset := pagination(c, 50)
	var bugs []qualitygorm.QualityBug
	order, err := qualityBugOrder(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	if query.Session(&gorm.Session{}).Order(order).Order("id DESC").
		Limit(limit).Offset(offset).Find(&bugs).Error != nil {
		dbFail(c)
		return
	}
	ctx := s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withQualityBugs(bugs),
	)
	rows := make([]gin.H, 0, len(bugs))
	for _, bug := range bugs {
		if _, ok := ctx.ProjectSets[bug.ProjectSetID]; !ok {
			continue
		}
		rows = append(rows, qualityBugDTO(bug, ctx))
	}
	ok(c, gin.H{"items": rows, "total": total})
}

type qualityBugAggregateCount struct {
	Status      string
	SeverityKey string
	Overdue     bool
	Count       int64
}

type qualityBugStatusCount struct {
	Status string
	Count  int64
}

type projectQualityOverviewCount struct {
	RequirementID uint
	SeverityKey   string
	Count         int64
}

func (s *Server) projectQualityOverview(c *gin.Context) {
	projectSetID, valid := paramID(c, "id")
	if !valid {
		return
	}
	workspace, _ := currentWorkspace(c)
	if !s.requireProjectAccess(c, workspace.ID, projectSetID) {
		return
	}
	projectIDs, err := parseUintListQuery(c, "project_ids")
	if err != nil {
		bad(c, err.Error())
		return
	}
	if err := s.validatePlanUnitScope(workspace.ID, []uint{projectSetID}, projectIDs); err != nil {
		bad(c, err.Error())
		return
	}
	query := s.db.Model(&qualitygorm.QualityBug{}).Where(
		"workspace_id = ? AND project_set_id = ?", workspace.ID, projectSetID,
	)
	if len(projectIDs) > 0 {
		query = query.Where("project_id IN ?", projectIDs)
	}
	query = applyWorkspaceHierarchyScope(query, "quality_bugs", workspace.ID).
		Where("LOWER(TRIM(status)) NOT IN ?", qualityBugStatusesStorageAliases("done", "canceled"))
	var counts []projectQualityOverviewCount
	if err := query.Select(
		"requirement_id, LOWER(TRIM(severity_key)) AS severity_key, COUNT(*) AS count",
	).Group("requirement_id, LOWER(TRIM(severity_key))").
		Order("requirement_id ASC, severity_key ASC").Scan(&counts).Error; err != nil {
		dbFail(c)
		return
	}
	items := make([]gin.H, 0, len(counts))
	total := int64(0)
	for _, row := range counts {
		total += row.Count
		items = append(items, gin.H{
			"requirement_id": row.RequirementID,
			"severity":       projectQualityOverviewSeverity(row.SeverityKey),
			"count":          row.Count,
		})
	}
	ok(c, gin.H{"items": items, "total": total})
}

func projectQualityOverviewSeverity(value string) string {
	value = normalizeStatus(value)
	if value == "p0" || value == "p1" || value == "critical" || value == "fatal" || value == "blocker" || value == "high" {
		return "critical"
	}
	if value == "p3" || value == "p4" || value == "p5" || value == "low" || value == "minor" || value == "trivial" {
		return "minor"
	}
	return "normal"
}

func (s *Server) qualityBugStatistics(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	scope, err := s.parseWorkspaceScope(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	filtered := applyWorkspaceScope(
		s.db.Model(&qualitygorm.QualityBug{}).Where("workspace_id = ?", workspace.ID), scope,
	)
	filtered = applyWorkspaceHierarchyScope(filtered, "quality_bugs", workspace.ID)
	s.applyProjectDataFilter(c, filtered, "quality_bugs", "project_set_id")
	filtered, err = applyQualityBugFilters(c, filtered, true)
	if err != nil {
		bad(c, err.Error())
		return
	}
	var counts []qualityBugAggregateCount
	if err := filtered.Select(
		"LOWER(TRIM(status)) AS status, LOWER(TRIM(severity_key)) AS severity_key, " +
			"CASE WHEN due_at IS NOT NULL AND due_at < CURRENT_TIMESTAMP THEN true ELSE false END AS overdue, COUNT(*) AS count",
	).Group(
		"LOWER(TRIM(status)), LOWER(TRIM(severity_key)), " +
			"CASE WHEN due_at IS NOT NULL AND due_at < CURRENT_TIMESTAMP THEN true ELSE false END",
	).Scan(&counts).Error; err != nil {
		dbFail(c)
		return
	}

	facetQuery := applyWorkspaceScope(
		s.db.Model(&qualitygorm.QualityBug{}).Where("workspace_id = ?", workspace.ID), scope,
	)
	facetQuery = applyWorkspaceHierarchyScope(facetQuery, "quality_bugs", workspace.ID)
	s.applyProjectDataFilter(c, facetQuery, "quality_bugs", "project_set_id")
	facetQuery, err = applyQualityBugFilters(c, facetQuery, false)
	if err != nil {
		bad(c, err.Error())
		return
	}
	var facetCounts []qualityBugStatusCount
	if err := facetQuery.Select("LOWER(TRIM(status)) AS status, COUNT(*) AS count").
		Group("LOWER(TRIM(status))").Scan(&facetCounts).Error; err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"summary": qualityBugSummaryFromCounts(counts),
		"facets":  gin.H{"status": qualityBugStatusFacetsFromCounts(facetCounts)},
	})
}

func qualityBugSummaryFromCounts(rows []qualityBugAggregateCount) gin.H {
	summary := gin.H{
		"total": int64(0), "open": int64(0), "discovery": int64(0),
		"handling": int64(0), "review": int64(0), "closed": int64(0),
		"high_severity": int64(0), "overdue": int64(0),
	}
	for _, row := range rows {
		status := normalizeQualityBugStatus(row.Status)
		terminal := status == "done" || status == "canceled"
		summary["total"] = summary["total"].(int64) + row.Count
		if !terminal {
			summary["open"] = summary["open"].(int64) + row.Count
			if row.Overdue {
				summary["overdue"] = summary["overdue"].(int64) + row.Count
			}
		}
		if status == "triage" || status == "todo" {
			summary["discovery"] = summary["discovery"].(int64) + row.Count
		}
		if status == "in_progress" || status == "reopened" {
			summary["handling"] = summary["handling"].(int64) + row.Count
		}
		if status == "review" {
			summary["review"] = summary["review"].(int64) + row.Count
		}
		if terminal {
			summary["closed"] = summary["closed"].(int64) + row.Count
		}
		if row.SeverityKey == "p0" || row.SeverityKey == "p1" {
			summary["high_severity"] = summary["high_severity"].(int64) + row.Count
		}
	}
	return summary
}

func qualityBugStatusFacetsFromCounts(rows []qualityBugStatusCount) gin.H {
	result := gin.H{
		"todo": int64(0), "triage": int64(0), "in_progress": int64(0),
		"reopened": int64(0), "review": int64(0), "done": int64(0),
		"canceled": int64(0), "closed": int64(0),
	}
	for _, row := range rows {
		status := normalizeQualityBugStatus(row.Status)
		if _, exists := result[status]; exists {
			result[status] = result[status].(int64) + row.Count
		}
		if status == "done" || status == "canceled" {
			result["closed"] = result["closed"].(int64) + row.Count
		}
	}
	return result
}

func applyQualityBugFilters(c *gin.Context, query *gorm.DB, includeStatus bool) (*gorm.DB, error) {
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		like := escapedLikePattern(q)
		query = query.Where("LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(bug_key) LIKE ? ESCAPE '\\'", like, like)
	}
	if values := qualityCSVQuery(c, "status", "statuses"); includeStatus && len(values) > 0 {
		query = query.Where("LOWER(TRIM(status)) IN ?", qualityBugStatusesStorageAliases(values...))
	}
	if values := qualityCSVQuery(c, "exclude_statuses"); includeStatus && len(values) > 0 {
		query = query.Where("(LOWER(TRIM(status)) NOT IN ? OR status IS NULL)", qualityBugStatusesStorageAliases(values...))
	}
	if values := qualityCSVQuery(c, "priority", "priorities"); len(values) > 0 {
		query = query.Where("LOWER(priority) IN ?", values)
	}
	if values := parseCSVStrings(c.Query("severity_key")); len(values) > 0 {
		query = query.Where("LOWER(severity_key) IN ?", values)
	}
	if environments := qualityCSVQuery(c, "found_environment", "found_environments"); len(environments) > 0 {
		query = query.Where("LOWER(TRIM(found_environment)) IN ?", environments)
	}
	if version := strings.TrimSpace(c.Query("found_version")); version != "" {
		query = query.Where("LOWER(found_version) = ?", strings.ToLower(version))
	}
	if version := strings.TrimSpace(c.Query("found_version_like")); version != "" {
		query = query.Where("LOWER(found_version) LIKE ? ESCAPE '\\'", escapedLikePattern(version))
	}
	if values := parseCSVStrings(c.Query("bug_type")); len(values) > 0 {
		query = query.Where("LOWER(bug_type) IN ?", values)
	}
	if values := qualityCSVQuery(c, "defect_domain", "defect_domains"); len(values) == 1 {
		if values[0] == quality.BugDefectDomainUI {
			query = query.Where("LOWER(TRIM(defect_domain)) = ?", quality.BugDefectDomainUI)
		} else if values[0] == quality.BugDefectDomainFunctional {
			query = query.Where(
				"defect_domain IS NULL OR TRIM(defect_domain) = '' OR LOWER(TRIM(defect_domain)) <> ?",
				quality.BugDefectDomainUI,
			)
		}
	}
	if ownerIDs := qualityUintCSVQuery(c, "current_owner_user_id", "current_owner_user_ids"); len(ownerIDs) > 0 {
		reviewerOwnedStatuses := qualityBugStatusesStorageAliases(
			"review", "done", "canceled",
		)
		query = applyQualityCurrentOwnerFilter(query, ownerIDs, reviewerOwnedStatuses)
	}
	if assigneeIDs := qualityUintCSVQuery(c, "assignee_user_id", "assignee_ids", "defect_owner_user_id"); len(assigneeIDs) > 0 {
		query = query.Where("assignee_user_id IN ?", assigneeIDs)
	}
	if reporterIDs := qualityUintCSVQuery(c, "reporter_user_id", "reporter_user_ids"); len(reporterIDs) > 0 {
		query = query.Where("reporter_user_id IN ?", reporterIDs)
	}
	actorID, reportedByMe, err := qualityActorFilter(c, "reported_by_me")
	if err != nil {
		return nil, err
	}
	if reportedByMe {
		query = query.Where("reporter_user_id = ?", actorID)
	}
	actorID, assignedToMe, err := qualityActorFilter(c, "assigned_to_me")
	if err != nil {
		return nil, err
	}
	if assignedToMe {
		reviewerOwnedStatuses := qualityBugStatusesStorageAliases("review", "done", "canceled")
		query = applyQualityCurrentOwnerFilter(query, []uint{actorID}, reviewerOwnedStatuses)
	}
	actorID, responsibleByMe, err := qualityActorFilter(c, "responsible_by_me")
	if err != nil {
		return nil, err
	}
	if responsibleByMe {
		query = query.Where("assignee_user_id = ?", actorID)
	}
	actorID, resolvedByMe, err := qualityActorFilter(c, "resolved_by_me")
	if err != nil {
		return nil, err
	}
	if resolvedByMe {
		query = query.Where(
			"assignee_user_id = ? AND LOWER(TRIM(status)) IN ?",
			actorID, qualityBugStatusesStorageAliases("review", "done"),
		)
	}
	createdFrom, fromSet, _, err := taskCreatedBoundary(c, "created_from")
	if err != nil {
		return nil, err
	}
	createdTo, toSet, toDateOnly, err := taskCreatedBoundary(c, "created_to")
	if err != nil {
		return nil, err
	}
	if fromSet {
		query = query.Where("quality_bugs.created_at >= ?", createdFrom)
	}
	if toSet {
		if toDateOnly {
			query = query.Where("quality_bugs.created_at < ?", createdTo.AddDate(0, 0, 1))
		} else {
			query = query.Where("quality_bugs.created_at <= ?", createdTo)
		}
	}
	if fromSet && toSet {
		upper := createdTo
		if toDateOnly {
			upper = upper.AddDate(0, 0, 1)
		}
		if createdFrom.After(upper) || createdFrom.Equal(upper) && toDateOnly {
			return nil, fmt.Errorf("created_from cannot be after created_to")
		}
	}
	return query, nil
}

func applyQualityCurrentOwnerFilter(query *gorm.DB, ownerIDs []uint, reviewerOwnedStatuses []string) *gorm.DB {
	return query.Where(
		"((LOWER(TRIM(status)) IN ? AND COALESCE(reviewer_user_id, reporter_user_id) IN ?) OR "+
			"((LOWER(TRIM(status)) NOT IN ? OR status IS NULL) AND assignee_user_id IN ?))",
		reviewerOwnedStatuses, ownerIDs, reviewerOwnedStatuses, ownerIDs,
	)
}

func qualityActorFilter(c *gin.Context, key string) (uint, bool, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0, false, nil
	}
	if value != "1" && !strings.EqualFold(value, "true") {
		return 0, false, fmt.Errorf("%s is invalid", key)
	}
	actorID := currentFactUserID(c)
	if actorID == 0 {
		return 0, false, fmt.Errorf("%s requires an authenticated actor", key)
	}
	return actorID, true, nil
}

func qualityBugOrder(c *gin.Context) (string, error) {
	direction := strings.ToUpper(strings.TrimSpace(c.DefaultQuery("sort_order", "desc")))
	if direction != "ASC" && direction != "DESC" {
		return "", fmt.Errorf("sort_order is invalid")
	}
	var expression string
	switch strings.TrimSpace(c.DefaultQuery("sort_by", "created_at")) {
	case "created_at":
		expression = "created_at"
	case "status_changed_at":
		expression = "status_changed_at"
	case "priority":
		expression = "CASE LOWER(priority) WHEN 'urgent' THEN 4 WHEN 'high' THEN 3 WHEN 'medium' THEN 2 WHEN 'low' THEN 1 ELSE 0 END"
	case "severity":
		expression = "CASE LOWER(severity_key) WHEN 'p0' THEN 4 WHEN 'p1' THEN 3 WHEN 'p2' THEN 2 WHEN 'p3' THEN 1 ELSE 0 END"
	case "status":
		expression = "CASE LOWER(status) WHEN 'triage' THEN 1 WHEN 'todo' THEN 2 WHEN 'in_progress' THEN 3 WHEN 'reopened' THEN 4 WHEN 'review' THEN 5 WHEN 'done' THEN 6 WHEN 'canceled' THEN 7 ELSE 8 END"
	default:
		return "", fmt.Errorf("sort_by is invalid")
	}
	return expression + " " + direction, nil
}

func qualityCSVQuery(c *gin.Context, keys ...string) []string {
	seen := map[string]struct{}{}
	values := make([]string, 0)
	for _, key := range keys {
		for _, raw := range c.Request.URL.Query()[key] {
			for _, value := range parseCSVStrings(raw) {
				if _, ok := seen[value]; ok {
					continue
				}
				seen[value] = struct{}{}
				values = append(values, value)
			}
		}
	}
	return values
}

func qualityUintCSVQuery(c *gin.Context, keys ...string) []uint {
	seen := map[uint]struct{}{}
	values := make([]uint, 0)
	for _, raw := range qualityCSVQuery(c, keys...) {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || value == 0 {
			continue
		}
		id := uint(value)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		values = append(values, id)
	}
	return values
}
