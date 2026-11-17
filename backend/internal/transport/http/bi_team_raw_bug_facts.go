package httpapi

import (
	"sort"
	"strconv"
	"strings"
	"time"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
)

func qualityActivitiesByBug(rows []quality.BugActivitySnapshot) map[uint][]quality.BugActivitySnapshot {
	result := make(map[uint][]quality.BugActivitySnapshot)
	for _, row := range rows {
		result[row.QualityBugID] = append(result[row.QualityBugID], row)
	}
	return result
}

func biTeamBugCloseInterval(
	bug quality.BugSnapshot,
	activities []quality.BugActivitySnapshot,
) (time.Time, time.Time, bool) {
	if bug.CompletedAt == nil || bug.ReviewerUserID == nil || bug.ResolvedAt == nil || bug.CompletedAt.Before(*bug.ResolvedAt) {
		return time.Time{}, time.Time{}, false
	}
	resolvedAt := *bug.ResolvedAt
	for _, activity := range activities {
		status := normalizeQualityBugStatus(activity.NewValue)
		if normalizeStatus(activity.FieldKey) == "status" && status == "review" &&
			!activity.CreatedAt.After(*bug.CompletedAt) && activity.CreatedAt.After(resolvedAt) {
			resolvedAt = activity.CreatedAt
		}
	}
	return resolvedAt, *bug.CompletedAt, true
}

func biTeamBugResolutionDecisionAt(
	bug quality.BugSnapshot,
	activities []quality.BugActivitySnapshot,
) *time.Time {
	var latest *time.Time
	for _, activity := range activities {
		if normalizeStatus(activity.FieldKey) != "resolution_reason" ||
			strings.TrimSpace(activity.NewValue) != strings.TrimSpace(bug.ResolutionReason) ||
			(latest != nil && !activity.CreatedAt.After(*latest)) {
			continue
		}
		at := activity.CreatedAt
		latest = &at
	}
	if latest != nil {
		return latest
	}
	for _, activity := range activities {
		if normalizeStatus(activity.FieldKey) == "status" &&
			normalizeQualityBugStatus(activity.NewValue) == "review" &&
			(latest == nil || activity.CreatedAt.After(*latest)) {
			at := activity.CreatedAt
			latest = &at
		}
	}
	if bug.ResolvedAt != nil && (latest == nil || bug.ResolvedAt.After(*latest)) {
		at := *bug.ResolvedAt
		latest = &at
	}
	if latest == nil {
		return nil
	}
	for _, activity := range activities {
		if activity.CreatedAt.After(*latest) && normalizeStatus(activity.FieldKey) == "multiple_fields" {
			return nil
		}
	}
	return latest
}

type biTeamBugResponseDuration struct {
	respondedAt time.Time
	hours       float64
}

func biTeamBugResponseDurations(
	bugs []quality.BugSnapshot,
	activities []quality.BugActivitySnapshot,
) map[uint]biTeamBugResponseDuration {
	byBug := qualityActivitiesByBug(activities)
	result := make(map[uint]biTeamBugResponseDuration)
	for _, bug := range bugs {
		if bug.AssigneeUserID == nil || *bug.AssigneeUserID == 0 || bug.CreatedAt.IsZero() {
			continue
		}
		rows := append([]quality.BugActivitySnapshot(nil), byBug[bug.ID]...)
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
				return rows[i].ID < rows[j].ID
			}
			return rows[i].CreatedAt.Before(rows[j].CreatedAt)
		})
		ownerText := strconv.FormatUint(uint64(*bug.AssigneeUserID), 10)
		var assignedAt *time.Time
		var respondedAt *time.Time
		for _, activity := range rows {
			field := normalizeStatus(activity.FieldKey)
			if field == "assignee_user_id" {
				assignedAt = nil
				respondedAt = nil
				if strings.TrimSpace(activity.NewValue) == ownerText {
					at := activity.CreatedAt
					assignedAt = &at
				}
				continue
			}
			status := normalizeQualityBugStatus(activity.NewValue)
			if assignedAt != nil && field == "status" && (status == "in_progress" || status == "review") &&
				!activity.CreatedAt.Before(*assignedAt) && (respondedAt == nil || activity.CreatedAt.Before(*respondedAt)) {
				at := activity.CreatedAt
				respondedAt = &at
			}
		}
		if assignedAt != nil && respondedAt == nil && bug.ResolvedAt != nil && !bug.ResolvedAt.Before(*assignedAt) {
			at := *bug.ResolvedAt
			respondedAt = &at
		}
		if assignedAt != nil && respondedAt != nil {
			result[bug.ID] = biTeamBugResponseDuration{
				respondedAt: *respondedAt, hours: respondedAt.Sub(*assignedAt).Hours(),
			}
		}
	}
	return result
}
