package biscreen

import (
	"sort"

	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
)

const (
	biTeamScoreTotalWeightBPS     = 10_000
	biTeamScoreCoreMinimumBPS     = 5_000
	biTeamScoreMinimumCoverageBPS = 7_000
	biTeamScoreMinimumWorkload    = 3
	biTeamScoreRoleAll            = "all"
	biTeamScoreMissingRenormalize = "renormalize"
	biTeamScoreMissingIneligible  = "ineligible"
)

var biTeamScoreRoles = []string{"product", "development", "testing"}

type biTeamMetricDefinition struct {
	Key               string
	RoleKey           string
	Label             string
	Description       string
	Formula           string
	Unit              string
	Direction         string
	Normalization     string
	DefinitionVersion int
	Status            string
	DefaultWeightBPS  int
	DefaultMinSamples int
	SortOrder         int
}

var biTeamMetricCatalog = []biTeamMetricDefinition{
	{Key: "workload_share", RoleKey: "product", Label: "需求量得分", Description: "本人负责需求数相对产品岗位人均需求数的比例，达到或超过人均值按满分计。", Formula: "本人负责需求数 ÷ 产品岗位人均需求数（100% 封顶）", Unit: "percent", Direction: "higher_better", Normalization: "role_average_share", DefinitionVersion: 1, Status: "active", DefaultWeightBPS: 3_500, DefaultMinSamples: 1, SortOrder: 5},
	{Key: "completion_rate", RoleKey: "product", Label: "需求完成率", Description: "本人负责需求的完成比例。", Formula: "已完成需求数 ÷ 本人负责需求数", Unit: "percent", Direction: "higher_better", Normalization: "percent", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 2_500, DefaultMinSamples: 1, SortOrder: 10},
	{Key: "on_time_rate", RoleKey: "product", Label: "关联任务按期完成率", Description: "本人负责需求下具备完整日期事实的任务按期完成比例。", Formula: "按期完成的关联任务数 ÷ 有截止时间和完成时间的关联任务数", Unit: "percent", Direction: "higher_better", Normalization: "percent", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 1_500, DefaultMinSamples: 1, SortOrder: 20},
	{Key: "first_pass_rate", RoleKey: "product", Label: "需求一次通过率", Description: "未发生任务返工的已交付需求比例。", Formula: "未发生返工的已交付需求数 ÷ 已进入交付的需求数", Unit: "percent", Direction: "higher_better", Normalization: "percent", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 1_000, DefaultMinSamples: 1, SortOrder: 30},
	{Key: "quality_health", RoleKey: "product", Label: "有效 Bug 需求占比", Description: "本人负责的需求中，至少关联一个非取消、非无效 Bug 的需求比例；数值越低越好。", Formula: "关联有效 Bug 的需求数 ÷ 本人负责需求数", Unit: "percent", Direction: "lower_better", Normalization: "inverse_percent", DefinitionVersion: 3, Status: "active", DefaultWeightBPS: 1_000, DefaultMinSamples: 1, SortOrder: 40},
	{Key: "response_efficiency", RoleKey: "product", Label: "平均需求首次拆解时长", Description: "本人负责的需求从创建到首个关联任务创建的平均小时数。", Formula: "Σ(首个关联任务创建时间 − 需求创建时间) ÷ 有首个任务的需求数", Unit: "hours", Direction: "lower_better", Normalization: "role_response_target", DefinitionVersion: 3, Status: "active", DefaultWeightBPS: 500, DefaultMinSamples: 1, SortOrder: 50},

	{Key: "workload_share", RoleKey: "development", Label: "任务量得分", Description: "本人负责任务数相对研发岗位人均任务数的比例，达到或超过人均值按满分计。", Formula: "本人负责任务数 ÷ 研发岗位人均任务数（100% 封顶）", Unit: "percent", Direction: "higher_better", Normalization: "role_average_share", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 3_500, DefaultMinSamples: 1, SortOrder: 110},
	{Key: "completion_rate", RoleKey: "development", Label: "任务完成率", Description: "本人负责研发任务的完成比例。", Formula: "本人已完成任务数 ÷ 本人负责任务数", Unit: "percent", Direction: "higher_better", Normalization: "percent", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 2_500, DefaultMinSamples: 1, SortOrder: 120},
	{Key: "on_time_rate", RoleKey: "development", Label: "任务按期完成率", Description: "本人具备完整日期事实的任务按期完成比例。", Formula: "按期完成任务数 ÷ 有截止时间和完成时间的任务数", Unit: "percent", Direction: "higher_better", Normalization: "percent", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 1_500, DefaultMinSamples: 1, SortOrder: 130},
	{Key: "quality_health", RoleKey: "development", Label: "Bug/任务率", Description: "指派给本人的有效 Bug 数与本人已完成任务数之比，数值越低越好。", Formula: "本人负责的有效 Bug 数 ÷ 本人已完成任务数", Unit: "percent", Direction: "lower_better", Normalization: "inverse_percent", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 1_000, DefaultMinSamples: 1, SortOrder: 140},
	{Key: "bug_response_rate", RoleKey: "development", Label: "Bug 及时响应率", Description: "指派后 24 小时内首次进入处理中的 Bug 比例。", Formula: "24 小时内首次进入处理中的 Bug 数 ÷ 已具备完整响应窗口的本人负责 Bug 数", Unit: "percent", Direction: "higher_better", Normalization: "percent", DefinitionVersion: 1, Status: "active", DefaultWeightBPS: 1_500, DefaultMinSamples: 1, SortOrder: 150},
	{Key: "first_pass_rate", RoleKey: "development", Label: "任务一次通过率（已停用）", Description: "旧版研发评分指标，仅用于兼容历史配置。", Formula: "未发生返工的已提交任务数 ÷ 已提交测试任务数", Unit: "percent", Direction: "higher_better", Normalization: "percent", DefinitionVersion: 1, Status: "deprecated", DefaultWeightBPS: 0, DefaultMinSamples: 1, SortOrder: 190},
	{Key: "response_efficiency", RoleKey: "development", Label: "平均任务响应时长（已停用）", Description: "旧版研发评分指标，仅用于兼容历史配置。", Formula: "Σ(首次进入开发中时间 − 任务创建时间) ÷ 有效任务样本数", Unit: "hours", Direction: "lower_better", Normalization: "role_response_target", DefinitionVersion: 1, Status: "deprecated", DefaultWeightBPS: 0, DefaultMinSamples: 1, SortOrder: 195},

	{Key: "workload_share", RoleKey: "testing", Label: "验证任务量得分", Description: "本人负责验证的 Bug 数相对测试岗位人均验证量的比例，达到或超过人均值按满分计。", Formula: "本人负责验证的 Bug 数 ÷ 测试岗位人均验证 Bug 数（100% 封顶）", Unit: "percent", Direction: "higher_better", Normalization: "role_average_share", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 3_500, DefaultMinSamples: 1, SortOrder: 210},
	{Key: "completion_rate", RoleKey: "testing", Label: "Bug 验证完成率", Description: "本人负责验证的 Bug 完成比例。", Formula: "已完成验证的 Bug 数 ÷ 本人负责验证的 Bug 数", Unit: "percent", Direction: "higher_better", Normalization: "percent", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 2_500, DefaultMinSamples: 1, SortOrder: 220},
	{Key: "on_time_rate", RoleKey: "testing", Label: "Bug 按期验证率", Description: "本人具备完整日期事实的 Bug 按期完成验证比例。", Formula: "按期完成验证的 Bug 数 ÷ 有截止时间和验证完成时间的 Bug 数", Unit: "percent", Direction: "higher_better", Normalization: "percent", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 1_500, DefaultMinSamples: 1, SortOrder: 230},
	{Key: "first_pass_rate", RoleKey: "testing", Label: "Bug 一次验证通过率", Description: "未发生回归退回的已验证 Bug 比例。", Formula: "未发生“回归验证 → 处理中”退回的 Bug 数 ÷ 已进入验证的 Bug 数", Unit: "percent", Direction: "higher_better", Normalization: "percent", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 1_000, DefaultMinSamples: 1, SortOrder: 240},
	{Key: "quality_health", RoleKey: "testing", Label: "无效 Bug 判定率", Description: "判定为重复、符合设计或外部原因的 Bug 比例，数值越低越好。", Formula: "无效 Bug 数 ÷ 已填写解决原因的 Bug 数", Unit: "percent", Direction: "lower_better", Normalization: "inverse_percent", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 1_000, DefaultMinSamples: 1, SortOrder: 250},
	{Key: "response_efficiency", RoleKey: "testing", Label: "平均验证响应时长", Description: "Bug 解决到验证完成的平均小时数。", Formula: "Σ(验证完成时间 − Bug 解决时间) ÷ 有效 Bug 样本数", Unit: "hours", Direction: "lower_better", Normalization: "role_response_target", DefinitionVersion: 2, Status: "active", DefaultWeightBPS: 500, DefaultMinSamples: 1, SortOrder: 260},
}

func biTeamMetricDefinitionByKeyRole(key, roleKey string) (biTeamMetricDefinition, bool) {
	for _, definition := range biTeamMetricCatalog {
		if definition.Key == key && definition.RoleKey == roleKey {
			return definition, true
		}
	}
	return biTeamMetricDefinition{}, false
}

func defaultBITeamScoreProfile() biTeamScoreProfileDTO {
	items := make([]biTeamScoreProfileItemDTO, 0, len(biTeamMetricCatalog))
	for _, definition := range biTeamMetricCatalog {
		items = append(items, biTeamScoreItemDTO(definition, workbench.BITeamScoreProfileItemSnapshot{
			MetricKey: definition.Key, RoleKey: definition.RoleKey,
			Enabled: definition.DefaultWeightBPS > 0, WeightBPS: definition.DefaultWeightBPS,
			MissingPolicy:  biTeamScoreMissingRenormalize,
			MinimumSamples: definition.DefaultMinSamples, SortOrder: definition.SortOrder,
		}))
	}
	return biTeamScoreProfileDTO{
		Source: "default", TotalWeightBPS: biTeamScoreTotalWeightBPS,
		MinimumCoverageBPS: biTeamScoreMinimumCoverageBPS,
		MinimumWorkload:    biTeamScoreMinimumWorkload, Items: items,
	}
}

func biTeamScoreItemDTO(definition biTeamMetricDefinition, item workbench.BITeamScoreProfileItemSnapshot) biTeamScoreProfileItemDTO {
	return biTeamScoreProfileItemDTO{
		MetricKey: definition.Key, RoleKey: definition.RoleKey,
		Label: definition.Label, Description: definition.Description, Formula: definition.Formula,
		Unit: definition.Unit, Direction: definition.Direction, Normalization: definition.Normalization,
		ApplicableRoles: []string{definition.RoleKey}, DefinitionVersion: definition.DefinitionVersion,
		Status: definition.Status, Enabled: item.Enabled, WeightBPS: item.WeightBPS,
		WeightPct: biTeamRound(float64(item.WeightBPS) / 100), MissingPolicy: item.MissingPolicy,
		MinimumSamples: item.MinimumSamples, SortOrder: item.SortOrder,
	}
}

func mergeBITeamScoreProfile(profile workbench.BITeamScoreProfileSnapshot) biTeamScoreProfileDTO {
	byKey := make(map[string]workbench.BITeamScoreProfileItemSnapshot, len(profile.Items))
	legacy := make(map[string]workbench.BITeamScoreProfileItemSnapshot)
	for _, item := range profile.Items {
		if item.RoleKey == biTeamScoreRoleAll || item.RoleKey == "" {
			legacy[item.MetricKey] = item
			continue
		}
		byKey[item.RoleKey+":"+item.MetricKey] = item
	}
	result := defaultBITeamScoreProfile()
	result.ID, result.Version, result.Source = profile.ID, profile.Version, "workspace"
	for index, definition := range biTeamMetricCatalog {
		item, exists := byKey[definition.RoleKey+":"+definition.Key]
		if !exists {
			item, exists = legacy[definition.Key]
		}
		if !exists {
			item = workbench.BITeamScoreProfileItemSnapshot{
				MetricKey: definition.Key, RoleKey: definition.RoleKey,
				MissingPolicy:  biTeamScoreMissingRenormalize,
				MinimumSamples: definition.DefaultMinSamples, SortOrder: definition.SortOrder,
			}
		}
		item.RoleKey = definition.RoleKey
		result.Items[index] = biTeamScoreItemDTO(definition, item)
	}
	sort.SliceStable(result.Items, func(i, j int) bool { return result.Items[i].SortOrder < result.Items[j].SortOrder })
	return result
}
