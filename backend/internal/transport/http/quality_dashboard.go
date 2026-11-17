package httpapi

import (
	"sort"

	"github.com/gin-gonic/gin"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

type qualityDashboardCounts struct {
	Submitted int64
	Resolved  int64
	Closed    int64
	Backlog   int64
}

type qualityDashboardAggregateRow struct {
	Status         string
	SeverityKey    string
	AssigneeUserID *uint
	ReporterUserID *uint
	ReviewerUserID *uint
	Count          int64
}

func (s *Server) qualityBugDashboard(c *gin.Context) {
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
	s.applyProjectDataFilter(c, query, "quality_bugs", "project_set_id")
	query, err = applyQualityBugFilters(c, query, true)
	if err != nil {
		bad(c, err.Error())
		return
	}
	var groups []qualityDashboardAggregateRow
	if query.Select(
		"status",
		"severity_key",
		"assignee_user_id",
		"reporter_user_id",
		"reviewer_user_id",
		"COUNT(*) AS count",
	).Group(
		"status, severity_key, assignee_user_id, reporter_user_id, reviewer_user_id",
	).Scan(&groups).Error != nil {
		dbFail(c)
		return
	}

	refs := workspaceContextReferences{}
	for _, group := range groups {
		bug := qualitygorm.QualityBug{
			Status: group.Status, AssigneeUserID: group.AssigneeUserID,
			ReporterUserID: group.ReporterUserID, ReviewerUserID: group.ReviewerUserID,
		}
		if owner := qualityCurrentOwner(bug); owner != nil {
			refs = refs.withUserID(*owner)
		}
	}
	ctx := s.loadWorkspaceContext(workspace.ID, refs)
	personCounts := map[uint]*qualityDashboardCounts{}
	statusCounts := map[string]int64{}
	severityCounts := map[string]int64{}
	total := qualityDashboardCounts{}
	for _, group := range groups {
		bug := qualitygorm.QualityBug{
			Status: group.Status, AssigneeUserID: group.AssigneeUserID,
			ReporterUserID: group.ReporterUserID, ReviewerUserID: group.ReviewerUserID,
		}
		status := normalizeQualityBugStatus(group.Status)
		severity := normalizeStatus(group.SeverityKey)
		if severity != "p0" && severity != "p1" && severity != "p2" && severity != "p3" {
			severity = "unspecified"
		}
		statusCounts[status] += group.Count
		severityCounts[severity] += group.Count
		addQualityDashboardCounts(&total, status, group.Count)

		ownerID := uint(0)
		if owner := qualityCurrentOwner(bug); owner != nil {
			ownerID = *owner
		}
		if personCounts[ownerID] == nil {
			personCounts[ownerID] = &qualityDashboardCounts{}
		}
		addQualityDashboardCounts(personCounts[ownerID], status, group.Count)
	}

	ok(c, gin.H{
		"summary":     qualityDashboardCountsDTO(total),
		"by_person":   qualityDashboardPeople(personCounts, ctx),
		"by_status":   qualityDashboardStatusRows(statusCounts),
		"by_severity": qualityDashboardSeverityRows(severityCounts),
	})
}

func addQualityDashboardCounts(counts *qualityDashboardCounts, status string, amount int64) {
	counts.Submitted += amount
	switch status {
	case "review":
		counts.Resolved += amount
		counts.Backlog += amount
	case "done":
		counts.Resolved += amount
		counts.Closed += amount
	case "canceled":
		counts.Closed += amount
	default:
		counts.Backlog += amount
	}
}

func qualityDashboardCountsDTO(counts qualityDashboardCounts) gin.H {
	return gin.H{
		"submitted": counts.Submitted,
		"resolved":  counts.Resolved,
		"closed":    counts.Closed,
		"backlog":   counts.Backlog,
	}
}

func qualityDashboardPeople(
	counts map[uint]*qualityDashboardCounts,
	ctx workspaceContext,
) []gin.H {
	rows := make([]gin.H, 0, len(counts))
	for userID, count := range counts {
		name := "未分配"
		avatarURL := ""
		if userID > 0 {
			name = userDisplayName(ctx, &userID)
			avatarURL = userAvatarURL(ctx, &userID)
			if name == "" {
				name = retiredUserDisplayName(userID)
			}
		}
		row := qualityDashboardCountsDTO(*count)
		row["user_id"] = userID
		row["user_name"] = name
		row["avatar_url"] = avatarURL
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i]["submitted"].(int64)
		right := rows[j]["submitted"].(int64)
		if left != right {
			return left > right
		}
		return rows[i]["user_name"].(string) < rows[j]["user_name"].(string)
	})
	return rows
}

func qualityDashboardStatusRows(counts map[string]int64) []gin.H {
	items := []struct{ key, label string }{
		{key: "todo", label: "待处理"},
		{key: "triage", label: "待确认"},
		{key: "in_progress", label: "处理中"},
		{key: "reopened", label: "重新打开"},
		{key: "review", label: "待验收"},
		{key: "done", label: "已关闭"},
	}
	rows := make([]gin.H, 0, len(items))
	for _, item := range items {
		rows = append(rows, gin.H{"key": item.key, "label": item.label, "count": counts[item.key]})
	}
	return rows
}

func qualityDashboardSeverityRows(counts map[string]int64) []gin.H {
	items := []struct{ key, label string }{
		{key: "p0", label: "致命（P0）"},
		{key: "p1", label: "严重（P1）"},
		{key: "p2", label: "一般（P2）"},
		{key: "p3", label: "轻微（P3）"},
		{key: "unspecified", label: "未设置"},
	}
	rows := make([]gin.H, 0, len(items))
	for _, item := range items {
		rows = append(rows, gin.H{"key": item.key, "label": item.label, "count": counts[item.key]})
	}
	return rows
}
