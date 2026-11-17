package codecontrib

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SyncService runs the collection loop: provider APIs → normalized facts →
// binding resolution → daily aggregates, per repository, idempotently.
//
// 采集模型（定时、可预期）：
//   - 接入后首次采集拉全量：所有 refs、全部历史，无窗口；
//   - 之后每个工作日 09:00（工作区时区）同步一个自然窗——日常同步昨天，
//     周一同步上周，每月 1 号同步上月（同日命中多重身份取更早边界）；
//   - 窗口起点同时不晚于上次成功采集日，故障恢复后自动补齐断档。
//
// 一致性规则：只有当提交与请求两侧都采集成功才落库；失败写入同步状态
// （数据新鲜度），绝不能让采集失败渲染成零贡献。窗口按工作区时区对齐到
// 自然日，因此"按日重写聚合"永远覆盖完整的一天。
type SyncService struct {
	deps SyncDependencies
	now  func() time.Time
}

type SyncDependencies struct {
	Connections  ConnectionStore
	Repositories RepositoryStore
	Settings     SettingsStore
	Bindings     BindingStore
	Facts        FactStore
	Collectors   map[Provider]Collector
	Members      MemberCandidateSource
	// Now 是可选的测试时钟；缺省 time.Now。
	Now func() time.Time
}

func NewSyncService(deps SyncDependencies) (*SyncService, error) {
	if deps.Connections == nil || deps.Repositories == nil || deps.Settings == nil ||
		deps.Bindings == nil || deps.Facts == nil || deps.Members == nil {
		return nil, ErrAdminInvalid
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &SyncService{deps: deps, now: now}, nil
}

// WorkspaceSyncResult is one workspace pass; per-repo failures are recorded
// in outcomes instead of aborting the whole workspace (fail-visible).
type WorkspaceSyncResult struct {
	RepoOutcomes []RepoSyncOutcome
}

type RepoSyncOutcome struct {
	RepoID       uint
	Path         string
	Commits      int
	PullRequests int
	Error        string
}

// syncSchedule 是工作区可配置的同步时机：每日/每周/每月三个独立触发时刻
// （工作区时区，精确到分钟），触发时同步对应自然窗（昨天/上周/上月）。
type syncSchedule struct {
	dailyHour, dailyMinute                 int
	weeklyWeekday                          time.Weekday
	weeklyHour, weeklyMinute               int
	monthlyDay, monthlyHour, monthlyMinute int
}

// scheduleFromConfig 读取口径配置并归一化：历史行/未配置字段回退到
// 09:00、周一、1 号的历史行为，越界值同样收敛，保证触发判定总有定义。
func scheduleFromConfig(config CollectionConfig) syncSchedule {
	hour, minute, weekday, monthDay := 9, 0, time.Monday, 1
	if config.DailySyncHour >= 0 && config.DailySyncHour <= 23 {
		hour = config.DailySyncHour
	}
	if config.DailySyncMinute >= 0 && config.DailySyncMinute <= 59 {
		minute = config.DailySyncMinute
	}
	if config.WeeklySyncWeekday >= 0 && config.WeeklySyncWeekday <= 6 {
		weekday = time.Weekday(config.WeeklySyncWeekday)
	}
	weeklyHour, weeklyMinute := hour, minute
	if config.WeeklySyncHour >= 0 && config.WeeklySyncHour <= 23 {
		weeklyHour = config.WeeklySyncHour
	}
	if config.WeeklySyncMinute >= 0 && config.WeeklySyncMinute <= 59 {
		weeklyMinute = config.WeeklySyncMinute
	}
	monthlyHour, monthlyMinute := hour, minute
	if config.MonthlySyncHour >= 0 && config.MonthlySyncHour <= 23 {
		monthlyHour = config.MonthlySyncHour
	}
	if config.MonthlySyncMinute >= 0 && config.MonthlySyncMinute <= 59 {
		monthlyMinute = config.MonthlySyncMinute
	}
	if config.MonthlySyncDay >= 1 && config.MonthlySyncDay <= 28 {
		monthDay = config.MonthlySyncDay
	}
	return syncSchedule{
		dailyHour: hour, dailyMinute: minute,
		weeklyWeekday: weekday, weeklyHour: weeklyHour, weeklyMinute: weeklyMinute,
		monthlyDay: monthDay, monthlyHour: monthlyHour, monthlyMinute: monthlyMinute,
	}
}

// atTime 今天 location 时区的某个触发时刻。
func (schedule syncSchedule) atTime(now time.Time, location *time.Location, hour, minute int) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, location)
}

// SyncKind 区分一轮采集的触发身份：接入后的首次全量，或定时自然窗。
type SyncKind string

const (
	SyncKindFull      SyncKind = "full"
	SyncKindScheduled SyncKind = "scheduled"
)

// syncTarget 描述一轮待执行的采集。
type syncTarget struct {
	kind   SyncKind
	window Window
}

// SyncWorkspace immediately syncs every enabled repository of one workspace
// with its appropriate target（首次全量，否则当前自然窗）.
func (service *SyncService) SyncWorkspace(ctx context.Context, workspaceID uint) (WorkspaceSyncResult, error) {
	repositories, err := service.deps.Repositories.ListRepositories(ctx, workspaceID)
	if err != nil {
		return WorkspaceSyncResult{}, err
	}
	result := WorkspaceSyncResult{RepoOutcomes: make([]RepoSyncOutcome, 0, len(repositories))}
	for _, repository := range repositories {
		if !repository.Enabled {
			continue
		}
		config, err := service.effectiveSettings(ctx, workspaceID)
		if err != nil {
			return WorkspaceSyncResult{}, err
		}
		location := aggregationLocation(config.Timezone)
		state, _, err := service.deps.Facts.GetSyncRun(ctx, repository.ID)
		if err != nil {
			return WorkspaceSyncResult{}, err
		}
		target := syncTargetFor(service.now().In(location), location, state, scheduleFromConfig(config))
		outcome := service.SyncRepository(ctx, repository, config, target)
		result.RepoOutcomes = append(result.RepoOutcomes, outcome)
	}
	return result, nil
}

// SyncDueWorkspaces runs every enabled repository whose trigger has arrived:
// a pending initial full pull, or the workspace's configured daily scheduled
// window not yet successfully run today. 每分钟被 worker 调用，触发判定幂等。
func (service *SyncService) SyncDueWorkspaces(ctx context.Context, now time.Time) (WorkspaceSyncResult, error) {
	repositories, err := service.deps.Repositories.ListEnabledRepositories(ctx)
	if err != nil {
		return WorkspaceSyncResult{}, err
	}
	result := WorkspaceSyncResult{RepoOutcomes: make([]RepoSyncOutcome, 0, len(repositories))}
	for _, repository := range repositories {
		config, err := service.effectiveSettings(ctx, repository.WorkspaceID)
		if err != nil {
			return WorkspaceSyncResult{}, err
		}
		location := aggregationLocation(config.Timezone)
		state, _, err := service.deps.Facts.GetSyncRun(ctx, repository.ID)
		if err != nil {
			return WorkspaceSyncResult{}, err
		}
		target, due := syncTargetForDue(now.In(location), location, state, scheduleFromConfig(config))
		if !due {
			continue
		}
		result.RepoOutcomes = append(result.RepoOutcomes, service.SyncRepository(ctx, repository, config, target))
	}
	return result, nil
}

// syncTargetFor 计算仓库当前应执行的采集：未做过首次全量则全量，否则取
// 当前定时自然窗。
func syncTargetFor(now time.Time, location *time.Location, state RepoSyncState, schedule syncSchedule) syncTarget {
	if state.FullSyncedAt == nil {
		return syncTarget{kind: SyncKindFull, window: Window{Until: now}}
	}
	return syncTarget{kind: SyncKindScheduled, window: scheduledWindow(now, location, state, schedule)}
}

// syncTargetForDue 叠加定时触发条件：全量未做必做；否则今天任一适用触发
// 时刻（每日恒适用；周/月仅当日历日匹配时适用）已过、且上次成功早于该时刻
// 时触发（同一触发时刻当天重复调用幂等）。
func syncTargetForDue(now time.Time, location *time.Location, state RepoSyncState, schedule syncSchedule) (syncTarget, bool) {
	target := syncTargetFor(now, location, state, schedule)
	if target.kind == SyncKindFull {
		return target, true
	}
	triggers := []time.Time{
		schedule.atTime(now, location, schedule.dailyHour, schedule.dailyMinute),
	}
	if now.Weekday() == schedule.weeklyWeekday {
		triggers = append(triggers, schedule.atTime(now, location, schedule.weeklyHour, schedule.weeklyMinute))
	}
	if now.Day() == schedule.monthlyDay {
		triggers = append(triggers, schedule.atTime(now, location, schedule.monthlyHour, schedule.monthlyMinute))
	}
	for _, trigger := range triggers {
		if now.Before(trigger) {
			continue
		}
		if state.LastSuccessAt != nil && !state.LastSuccessAt.Before(trigger) {
			continue
		}
		return target, true
	}
	return syncTarget{}, false
}

// scheduledWindow 计算定时自然窗：起点取「已过触发时刻的适用自然窗中最早
// 边界」与「上次成功采集日」的更早者——每日窗是昨天，周/月窗仅当触发时刻
// 已过才并入（避免每日轮提前把整周拉走），后者让故障恢复自动补上断档。
func scheduledWindow(now time.Time, location *time.Location, state RepoSyncState, schedule syncSchedule) Window {
	boundary := dayStartIn(location, now.AddDate(0, 0, -1)) // 昨天
	if now.Weekday() == schedule.weeklyWeekday && !now.Before(schedule.atTime(now, location, schedule.weeklyHour, schedule.weeklyMinute)) {
		// 每周触发时刻已过：上周 = [7 天前 0 点, 今天 0 点)。
		weekly := dayStartIn(location, now.AddDate(0, 0, -7))
		if weekly.Before(boundary) {
			boundary = weekly
		}
	}
	if now.Day() == schedule.monthlyDay && !now.Before(schedule.atTime(now, location, schedule.monthlyHour, schedule.monthlyMinute)) {
		// 每月触发时刻已过：上月 = [上月 1 号 0 点, 今天 0 点)。
		firstOfLastMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location).AddDate(0, -1, 0)
		if firstOfLastMonth.Before(boundary) {
			boundary = firstOfLastMonth
		}
	}
	if state.LastSuccessAt != nil {
		if lastSuccess := dayStartIn(location, *state.LastSuccessAt); lastSuccess.Before(boundary) {
			boundary = lastSuccess
		}
	}
	return Window{Since: boundary.UTC(), Until: now}
}

func dayStartIn(location *time.Location, at time.Time) time.Time {
	local := at.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

// SyncRepository collects, binds and aggregates one repository for one
// target. 全量轮不设翻页上限；定时窗仍受 PageCap 约束。
func (service *SyncService) SyncRepository(ctx context.Context, repository RepositoryConfig, config CollectionConfig, target syncTarget) RepoSyncOutcome {
	outcome := RepoSyncOutcome{RepoID: repository.ID, Path: repository.Path}
	connection, err := service.deps.Connections.GetConnection(ctx, repository.WorkspaceID, repository.ConnectionID)
	if err != nil {
		return service.fail(ctx, repository, RepoSyncState{}, outcome, fmt.Errorf("连接不可用: %w", err))
	}
	collector, ok := service.deps.Collectors[connection.Provider]
	if !ok {
		return service.fail(ctx, repository, RepoSyncState{}, outcome, ErrAdminUnsupported)
	}
	state, _, err := service.deps.Facts.GetSyncRun(ctx, repository.ID)
	if err != nil {
		return service.fail(ctx, repository, state, outcome, err)
	}
	window := target.window

	pageCap := config.PageCap
	if target.kind == SyncKindFull {
		pageCap = 0
	}

	credentials := Credentials{Token: connection.Token, BaseURL: connection.BaseURL}
	repositoryRefValue := repositoryRef(repository, connection.Provider)
	commits, err := service.collectCommits(ctx, collector, credentials, repositoryRefValue, window, pageCap)
	if err != nil {
		return service.fail(ctx, repository, state, outcome, err)
	}
	requests, err := service.collectPullRequests(ctx, collector, credentials, repositoryRefValue, window, pageCap)
	if err != nil {
		return service.fail(ctx, repository, state, outcome, err)
	}

	classifyFacts(repository.FilterRules, commits, requests)
	MarkPRDelivered(commits, requests)
	if err := service.deps.Facts.UpsertCommits(ctx, repository.WorkspaceID, repository.ID, commits); err != nil {
		return service.fail(ctx, repository, state, outcome, err)
	}
	if err := service.deps.Facts.UpsertPullRequests(ctx, repository.WorkspaceID, repository.ID, requests); err != nil {
		return service.fail(ctx, repository, state, outcome, err)
	}

	bindingContext, err := service.bindingContext(ctx, repository.WorkspaceID, config)
	if err != nil {
		return service.fail(ctx, repository, state, outcome, err)
	}
	for _, claim := range collectIdentityClaims(connection.Provider, commits, requests) {
		row := bindingRowFor(connection.Provider, claim, ResolveAuthorIdentity(bindingContext, claim.identity))
		row.WorkspaceID = repository.WorkspaceID
		if err := service.deps.Bindings.UpsertBinding(ctx, row); err != nil {
			return service.fail(ctx, repository, state, outcome, err)
		}
	}

	resolve := func(identity AuthorIdentity) Resolution { return ResolveAuthorIdentity(bindingContext, identity) }
	aggregation := BuildDailyRows(repository.ID, commits, requests, resolve, config, service.now())
	rows := make([]DailyContributionRow, 0, len(aggregation.Rows))
	for _, row := range aggregation.Rows {
		row.WorkspaceID = repository.WorkspaceID
		rows = append(rows, row)
	}
	if err := service.deps.Facts.ReplaceDailyWindow(ctx, repository.WorkspaceID, repository.ID,
		aggregation.TouchedDays, rows); err != nil {
		return service.fail(ctx, repository, state, outcome, err)
	}

	service.saveSuccess(ctx, state, repository, target.kind, service.now())
	outcome.Commits = len(commits)
	outcome.PullRequests = len(requests)
	return outcome
}

// ReaggregateWorkspace rebuilds aggregates from stored facts with the
// current bindings — the binding confirm queue calls it so a confirmation
// reaches the report immediately. 重算覆盖全部已存历史：绑定是对全时段
// 事实的归属修正，报表严格按期统计，任何一天的归属都必须跟随最新绑定；
// 原始事实不可变，全量重放成本可接受。
func (service *SyncService) ReaggregateWorkspace(ctx context.Context, workspaceID uint) error {
	repositories, err := service.deps.Repositories.ListRepositories(ctx, workspaceID)
	if err != nil {
		return err
	}
	config, err := service.effectiveSettings(ctx, workspaceID)
	if err != nil {
		return err
	}
	bindingContext, err := service.bindingContext(ctx, workspaceID, config)
	if err != nil {
		return err
	}
	window := Window{Until: service.now()}
	for _, repository := range repositories {
		if !repository.Enabled {
			continue
		}
		commits, err := service.deps.Facts.ListCommitsInWindow(ctx, workspaceID, repository.ID, window)
		if err != nil {
			return err
		}
		requests, err := service.deps.Facts.ListMergedPullRequestsInWindow(ctx, workspaceID, repository.ID, window)
		if err != nil {
			return err
		}
		resolve := func(identity AuthorIdentity) Resolution { return ResolveAuthorIdentity(bindingContext, identity) }
		aggregation := BuildDailyRows(repository.ID, commits, requests, resolve, config, window.Until)
		rows := make([]DailyContributionRow, 0, len(aggregation.Rows))
		for _, row := range aggregation.Rows {
			row.WorkspaceID = workspaceID
			rows = append(rows, row)
		}
		if err := service.deps.Facts.ReplaceDailyWindow(ctx, workspaceID, repository.ID,
			aggregation.TouchedDays, rows); err != nil {
			return err
		}
	}
	return nil
}

func (service *SyncService) effectiveSettings(ctx context.Context, workspaceID uint) (CollectionConfig, error) {
	config, exists, err := service.deps.Settings.GetSettings(ctx, workspaceID)
	if err != nil {
		return CollectionConfig{}, err
	}
	if !exists {
		return defaultCollectionConfig(), nil
	}
	return config, nil
}

func (service *SyncService) bindingContext(ctx context.Context, workspaceID uint, config CollectionConfig) (BindingContext, error) {
	rows, err := service.deps.Bindings.ListBindingRows(ctx, workspaceID)
	if err != nil {
		return BindingContext{}, err
	}
	members, err := service.deps.Members.FindActiveMemberCandidates(ctx, workspaceID)
	if err != nil {
		return BindingContext{}, err
	}
	return BindingContextFor(config, rows, members), nil
}

func (service *SyncService) collectCommits(ctx context.Context, collector Collector,
	credentials Credentials, ref RepoRef, window Window, pageCap int,
) ([]CommitFact, error) {
	commits := make([]CommitFact, 0, 64)
	cursor := ""
	for page := 0; pageCap <= 0 || page < pageCap; page++ {
		batch, err := collector.ListCommits(ctx, credentials, ref, window, cursor)
		if err != nil {
			return nil, err
		}
		commits = append(commits, batch.Facts...)
		if !batch.HasMore || batch.NextCursor == "" {
			return commits, nil
		}
		cursor = batch.NextCursor
	}
	return commits, nil
}

func (service *SyncService) collectPullRequests(ctx context.Context, collector Collector,
	credentials Credentials, ref RepoRef, window Window, pageCap int,
) ([]PullRequestFact, error) {
	requests := make([]PullRequestFact, 0, 32)
	cursor := ""
	for page := 0; pageCap <= 0 || page < pageCap; page++ {
		batch, err := collector.ListMergedPullRequests(ctx, credentials, ref, window, cursor)
		if err != nil {
			return nil, err
		}
		requests = append(requests, batch.Facts...)
		if !batch.HasMore || batch.NextCursor == "" {
			return requests, nil
		}
		cursor = batch.NextCursor
	}
	return requests, nil
}

func repositoryRef(repository RepositoryConfig, provider Provider) RepoRef {
	return RepoRef{
		Provider:      provider,
		ExternalID:    repository.ExternalID,
		Path:          repository.Path,
		DefaultBranch: repository.DefaultBranch,
	}
}

// classifyFills fills every file's work type from the effective rules;
// binary blobs are classified directly by the collector's report.
func classifyFacts(rules FilterRules, commits []CommitFact, requests []PullRequestFact) {
	apply := func(files []FileChange) {
		for index := range files {
			if files[index].Binary {
				files[index].Class = FileClassBinary
				continue
			}
			files[index].Class = classifyPath(rules, files[index].Path)
		}
	}
	for index := range commits {
		apply(commits[index].Files)
	}
	for index := range requests {
		apply(requests[index].Files)
	}
}

type identityClaim struct {
	identity AuthorIdentity
	kind     string
	key      string
}

// collectIdentityClaims gathers the distinct identities the pipeline must
// judge, preferring email keys for commit authors and login keys for request
// authors and reviewers (email rows only when no login exists).
func collectIdentityClaims(provider Provider, commits []CommitFact, requests []PullRequestFact) []identityClaim {
	claims := make([]identityClaim, 0, 8)
	seen := make(map[string]struct{}, 8)
	add := func(identity AuthorIdentity, kind, key string) {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			return
		}
		id := string(provider) + ":" + kind + ":" + key
		if _, duplicate := seen[id]; duplicate {
			return
		}
		seen[id] = struct{}{}
		claims = append(claims, identityClaim{identity: identity, kind: kind, key: key})
	}
	for _, commit := range commits {
		if commit.AuthorEmail != "" {
			add(commitAuthorIdentity(commit), "email", commit.AuthorEmail)
		} else if commit.AuthorLogin != "" {
			add(commitAuthorIdentity(commit), "login", commit.AuthorLogin)
		}
		for _, email := range commit.CoAuthorEmails {
			add(AuthorIdentity{Email: email}, "email", email)
		}
	}
	for _, request := range requests {
		if request.AuthorLogin != "" {
			add(prAuthorIdentity(request), "login", request.AuthorLogin)
		} else if request.AuthorEmail != "" {
			add(prAuthorIdentity(request), "email", request.AuthorEmail)
		}
		for _, review := range request.Reviews {
			if review.ReviewerLogin != "" {
				add(AuthorIdentity{Login: review.ReviewerLogin}, "login", review.ReviewerLogin)
			} else if review.ReviewerEmail != "" {
				add(AuthorIdentity{Email: review.ReviewerEmail}, "email", review.ReviewerEmail)
			}
		}
	}
	sort.Slice(claims, func(left, right int) bool { return claims[left].key < claims[right].key })
	return claims
}

// bindingRowFor projects one resolution onto the persisted mapping row.
// DisplayName 带上 Git 作者名（提交作者名），确认队列直接显示人名。
func bindingRowFor(provider Provider, claim identityClaim, resolution Resolution) BindingRow {
	row := BindingRow{
		Provider:    provider,
		KeyKind:     claim.kind,
		AuthorKey:   claim.key,
		DisplayName: TruncateUTF8(strings.TrimSpace(claim.identity.Name), 255),
	}
	switch resolution.Status {
	case BindingMatched:
		row.Status = BindingMatched
		userID := resolution.UserID
		row.UserID = &userID
		row.MatchedVia = resolution.MatchedVia
	case BindingBot:
		row.Status = BindingBot
	case BindingSuggested:
		row.Status = BindingSuggested
		suggested := resolution.SuggestedUserID
		row.SuggestedUserID = &suggested
	default:
		row.Status = BindingUnmatched
	}
	return row
}

func (service *SyncService) saveSuccess(ctx context.Context, state RepoSyncState, repository RepositoryConfig,
	kind SyncKind, at time.Time,
) {
	state.WorkspaceID = repository.WorkspaceID
	state.RepoID = repository.ID
	state.LastSyncedAt = &at
	state.LastSuccessAt = &at
	state.LastError = ""
	state.ConsecutiveFailures = 0
	if kind == SyncKindFull {
		state.FullSyncedAt = &at
	}
	_ = service.deps.Facts.SaveSyncRun(ctx, state)
}

func (service *SyncService) fail(ctx context.Context, repository RepositoryConfig, state RepoSyncState, outcome RepoSyncOutcome, err error) RepoSyncOutcome {
	outcome.Error = truncateError(err.Error())
	at := service.now()
	state.WorkspaceID = repository.WorkspaceID
	state.RepoID = repository.ID
	state.LastSyncedAt = &at
	state.LastError = outcome.Error
	state.ConsecutiveFailures++
	_ = service.deps.Facts.SaveSyncRun(ctx, state)
	return outcome
}

func truncateError(message string) string {
	return TruncateUTF8(message, 900)
}
