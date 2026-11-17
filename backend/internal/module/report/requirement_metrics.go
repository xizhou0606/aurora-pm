package report

import (
	"strings"

	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

// RequirementMetrics 汇总产品岗位的「需求数量」：以产品负责人
// （product_owner_user_ids）计。需求快照没有完成时间字段，状态分布
// （已完成/验收中/开发中/待评审）是当前快照口径，「本期新增」按
// created_at 落在统计周期内计。这是产品岗位汇报的核心事实面板与 AI
// 评语的主要依据。
type RequirementMetrics struct {
	Total           int `json:"total"`
	CreatedInPeriod int `json:"created_in_period"`
	Done            int `json:"done"`
	Acceptance      int `json:"acceptance"`
	Development     int `json:"development"`
	// Pending 汇总 grooming 与 backlog 两个未进入开发的池内状态。
	Pending int `json:"pending"`
}

// BucketRequirementsByProductOwner partitions requirement snapshots once per
// workspace pass by product owner. One requirement co-owned by N people lands
// in N buckets: personal attribution counts it for every owner, while team
// aggregates deduplicate by requirement ID.
func BucketRequirementsByProductOwner(snapshots []requirement.Snapshot) map[uint][]requirement.Snapshot {
	buckets := make(map[uint][]requirement.Snapshot)
	for _, snapshot := range snapshots {
		for _, ownerID := range snapshot.ProductOwnerUserIDs {
			if ownerID == 0 {
				continue
			}
			buckets[ownerID] = append(buckets[ownerID], snapshot)
		}
	}
	return buckets
}

// aggregateRequirementMetrics counts one person's owned requirements: the
// total snapshot, the ones created inside the period, and the current status
// distribution.
func aggregateRequirementMetrics(snapshots []requirement.Snapshot, period Period) RequirementMetrics {
	metrics := RequirementMetrics{}
	for _, snapshot := range snapshots {
		metrics.Total++
		if withinPeriod(&snapshot.CreatedAt, period) {
			metrics.CreatedInPeriod++
		}
		accumulateRequirementStatus(&metrics, snapshot.Status)
	}
	return metrics
}

// aggregateRequirementMetricsDistinct is the team-level counterpart: members'
// buckets are unioned and deduplicated by requirement ID so a requirement
// co-owned inside one team is never double counted.
func aggregateRequirementMetricsDistinct(buckets [][]requirement.Snapshot, period Period) RequirementMetrics {
	seen := make(map[uint]struct{})
	union := make([]requirement.Snapshot, 0)
	for _, bucket := range buckets {
		for _, snapshot := range bucket {
			if _, exists := seen[snapshot.ID]; exists {
				continue
			}
			seen[snapshot.ID] = struct{}{}
			union = append(union, snapshot)
		}
	}
	return aggregateRequirementMetrics(union, period)
}

func accumulateRequirementStatus(metrics *RequirementMetrics, status string) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done":
		metrics.Done++
	case "acceptance":
		metrics.Acceptance++
	case "development":
		metrics.Development++
	case "grooming", "backlog":
		metrics.Pending++
	}
}
