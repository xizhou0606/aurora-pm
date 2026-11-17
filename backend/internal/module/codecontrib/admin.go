package codecontrib

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// AdminService implements the workspace admin surface: connection/repository
// CRUD, connection verification, the binding confirm queue and the statistics
// calibration. Fail-closed rules: tokens are never echoed, a repository can
// only reference a connection of the same workspace, and binding confirmation
// only accepts workspace members.
type AdminService struct {
	deps AdminDependencies
	now  func() time.Time
}

type AdminDependencies struct {
	Connections  ConnectionStore
	Repositories RepositoryStore
	Settings     SettingsStore
	Bindings     BindingStore
	Collectors   map[Provider]Collector
	Members      MemberCandidateSource
	Reaggregator Reaggregator // optional: rebuilds aggregates after binding changes
}

// Reaggregator rebuilds daily aggregates once bindings change so confirmed
// mappings reach the report without waiting for the next sync.
type Reaggregator interface {
	ReaggregateWorkspace(ctx context.Context, workspaceID uint) error
}

var (
	ErrAdminInvalid     = errors.New("codecontrib admin input is invalid")
	ErrAdminNotFound    = errors.New("codecontrib record not found")
	ErrAdminUnsupported = errors.New("codecontrib provider is not configured")
)

func NewAdminService(deps AdminDependencies) (*AdminService, error) {
	if deps.Connections == nil || deps.Repositories == nil || deps.Settings == nil ||
		deps.Bindings == nil || deps.Members == nil {
		return nil, ErrAdminInvalid
	}
	return &AdminService{deps: deps, now: time.Now}, nil
}

type ConnectionInput struct {
	Name     string
	Provider Provider
	BaseURL  string
	Token    string // empty on update keeps the stored token
	Enabled  *bool
}

func (service *AdminService) ListConnections(ctx context.Context, workspaceID uint) ([]Connection, error) {
	return service.deps.Connections.ListConnections(ctx, workspaceID)
}

func (service *AdminService) CreateConnection(ctx context.Context, workspaceID uint, actorID uint, input ConnectionInput) (Connection, error) {
	if err := validateConnectionInput(input, true); err != nil {
		return Connection{}, err
	}
	connection := Connection{
		WorkspaceID: workspaceID, Name: strings.TrimSpace(input.Name),
		Provider: input.Provider, BaseURL: strings.TrimSpace(input.BaseURL),
		Token: strings.TrimSpace(input.Token), Enabled: enabledOrDefault(input.Enabled, true),
		CreatedBy: actorID, UpdatedBy: actorID, Version: 1,
	}
	return service.deps.Connections.CreateConnection(ctx, connection)
}

func (service *AdminService) UpdateConnection(ctx context.Context, workspaceID, id, actorID uint, input ConnectionInput) (Connection, error) {
	existing, err := service.deps.Connections.GetConnection(ctx, workspaceID, id)
	if err != nil {
		return Connection{}, mapAdminError(err)
	}
	if err := validateConnectionInput(input, false); err != nil {
		return Connection{}, err
	}
	existing.Name = strings.TrimSpace(input.Name)
	existing.BaseURL = strings.TrimSpace(input.BaseURL)
	if trimmed := strings.TrimSpace(input.Token); trimmed != "" {
		existing.Token = trimmed
	}
	existing.Enabled = enabledOrDefault(input.Enabled, existing.Enabled)
	existing.UpdatedBy = actorID
	existing.Version++
	return service.deps.Connections.UpdateConnection(ctx, existing)
}

func (service *AdminService) DeleteConnection(ctx context.Context, workspaceID, id uint) error {
	repositories, err := service.deps.Repositories.ListRepositories(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, repository := range repositories {
		if repository.ConnectionID == id {
			return fmt.Errorf("%w: 仍有仓库绑定在该连接上，请先解除绑定", ErrAdminInvalid)
		}
	}
	return mapAdminError(service.deps.Connections.DeleteConnection(ctx, workspaceID, id))
}

// TestConnection verifies the stored token with the provider's current-user
// endpoint. The token never appears in errors or results.
func (service *AdminService) TestConnection(ctx context.Context, workspaceID, id uint) (Account, error) {
	connection, err := service.deps.Connections.GetConnection(ctx, workspaceID, id)
	if err != nil {
		return Account{}, mapAdminError(err)
	}
	collector, ok := service.deps.Collectors[connection.Provider]
	if !ok {
		return Account{}, ErrAdminUnsupported
	}
	return collector.VerifyConnection(ctx, Credentials{Token: connection.Token, BaseURL: connection.BaseURL})
}

func (service *AdminService) ListAccessibleRepos(ctx context.Context, workspaceID, connectionID uint, cursor string) (RepositoryListing, error) {
	connection, err := service.deps.Connections.GetConnection(ctx, workspaceID, connectionID)
	if err != nil {
		return RepositoryListing{}, mapAdminError(err)
	}
	collector, ok := service.deps.Collectors[connection.Provider]
	if !ok {
		return RepositoryListing{}, ErrAdminUnsupported
	}
	reader, ok := collector.(DirectoryReader)
	if !ok {
		return RepositoryListing{}, ErrAdminUnsupported
	}
	return reader.ListAccessibleRepos(ctx, Credentials{Token: connection.Token, BaseURL: connection.BaseURL}, cursor)
}

type RepositoryInput struct {
	ConnectionID  uint
	ExternalID    string
	Path          string
	DefaultBranch string
	FilterRules   *FilterRules
	ProjectSetID  *uint
	Enabled       *bool
}

func (service *AdminService) ListRepositories(ctx context.Context, workspaceID uint) ([]RepositoryConfig, error) {
	return service.deps.Repositories.ListRepositories(ctx, workspaceID)
}

func (service *AdminService) CreateRepository(ctx context.Context, workspaceID uint, actorID uint, input RepositoryInput) (RepositoryConfig, error) {
	if _, err := service.deps.Connections.GetConnection(ctx, workspaceID, input.ConnectionID); err != nil {
		return RepositoryConfig{}, fmt.Errorf("%w: 连接不存在或不属于当前工作区", ErrAdminInvalid)
	}
	if strings.TrimSpace(input.Path) == "" {
		return RepositoryConfig{}, fmt.Errorf("%w: 仓库路径不能为空", ErrAdminInvalid)
	}
	config := RepositoryConfig{
		WorkspaceID: workspaceID, ConnectionID: input.ConnectionID,
		ExternalID: strings.TrimSpace(input.ExternalID), Path: strings.TrimSpace(input.Path),
		DefaultBranch: strings.TrimSpace(input.DefaultBranch),
		FilterRules:   derefFilterRules(input.FilterRules), ProjectSetID: input.ProjectSetID,
		Enabled:   enabledOrDefault(input.Enabled, true),
		CreatedBy: actorID, UpdatedBy: actorID, Version: 1,
	}
	return service.deps.Repositories.CreateRepository(ctx, config)
}

func (service *AdminService) UpdateRepository(ctx context.Context, workspaceID, id, actorID uint, input RepositoryInput) (RepositoryConfig, error) {
	existing, err := service.deps.Repositories.GetRepository(ctx, workspaceID, id)
	if err != nil {
		return RepositoryConfig{}, mapAdminError(err)
	}
	if input.ConnectionID != 0 && input.ConnectionID != existing.ConnectionID {
		if _, err := service.deps.Connections.GetConnection(ctx, workspaceID, input.ConnectionID); err != nil {
			return RepositoryConfig{}, fmt.Errorf("%w: 连接不存在或不属于当前工作区", ErrAdminInvalid)
		}
		existing.ConnectionID = input.ConnectionID
	}
	if input.Path != "" {
		existing.Path = strings.TrimSpace(input.Path)
	}
	if input.ExternalID != "" {
		existing.ExternalID = strings.TrimSpace(input.ExternalID)
	}
	if input.DefaultBranch != "" {
		existing.DefaultBranch = strings.TrimSpace(input.DefaultBranch)
	}
	if input.FilterRules != nil {
		existing.FilterRules = *input.FilterRules
	}
	if input.ProjectSetID != nil {
		existing.ProjectSetID = input.ProjectSetID
	}
	existing.Enabled = enabledOrDefault(input.Enabled, existing.Enabled)
	existing.UpdatedBy = actorID
	existing.Version++
	return service.deps.Repositories.UpdateRepository(ctx, existing)
}

func (service *AdminService) DeleteRepository(ctx context.Context, workspaceID, id uint) error {
	return mapAdminError(service.deps.Repositories.DeleteRepository(ctx, workspaceID, id))
}

func (service *AdminService) ListBindings(ctx context.Context, workspaceID uint, statuses []BindingStatus, limit int) ([]BindingRow, error) {
	rows, err := service.deps.Bindings.ListBindings(ctx, workspaceID, statuses, limit)
	if err != nil {
		return nil, err
	}
	// 读路径实时补建议：未归属的登录名若与某条已绑定邮箱的本地部分一致
	// （PR 作者登录名 ↔ 同人已绑定的提交邮箱），直接给出建议归属人，
	// 不必等下一轮采集重算；已有建议的行不覆盖。
	all, err := service.deps.Bindings.ListBindingRows(ctx, workspaceID)
	if err != nil {
		return rows, nil
	}
	matchedEmails := make([]AuthorBinding, 0, len(all))
	for _, row := range all {
		// UserID 非零即已确认绑定（admin/email_exact/login_exact）；
		// 未归属与建议行的 user_id 为空，天然被过滤。
		if row.UserID != 0 && row.AuthorEmail != "" {
			matchedEmails = append(matchedEmails, row)
		}
	}
	for index := range rows {
		if rows[index].Status != BindingUnmatched || rows[index].KeyKind != "login" || rows[index].SuggestedUserID != nil {
			continue
		}
		if userID, hit := SuggestUserForLogin(matchedEmails, rows[index].AuthorKey); hit {
			suggested := userID
			rows[index].SuggestedUserID = &suggested
		}
	}
	return rows, nil
}

// ConfirmBinding binds the row to a workspace member. The member check fails
// closed: an ID outside the workspace membership can never be bound.
func (service *AdminService) ConfirmBinding(ctx context.Context, workspaceID, id, userID, actorID uint) (BindingRow, error) {
	if !service.isMember(ctx, workspaceID, userID) {
		return BindingRow{}, fmt.Errorf("%w: 目标用户不是当前工作区成员", ErrAdminInvalid)
	}
	row, err := service.deps.Bindings.ConfirmBinding(ctx, workspaceID, id, userID, actorID, service.now())
	if err != nil {
		return BindingRow{}, mapAdminError(err)
	}
	service.reaggregate(ctx, workspaceID)
	return row, nil
}

func (service *AdminService) RejectBinding(ctx context.Context, workspaceID, id, actorID uint) (BindingRow, error) {
	row, err := service.deps.Bindings.RejectBinding(ctx, workspaceID, id, actorID)
	if err != nil {
		return BindingRow{}, mapAdminError(err)
	}
	service.reaggregate(ctx, workspaceID)
	return row, nil
}

func (service *AdminService) GetSettings(ctx context.Context, workspaceID uint) (CollectionConfig, error) {
	config, exists, err := service.deps.Settings.GetSettings(ctx, workspaceID)
	if err != nil {
		return CollectionConfig{}, err
	}
	if !exists {
		return defaultCollectionConfig(), nil
	}
	return config, nil
}

type SettingsInput struct {
	Timezone    string
	BotPatterns []string
	PageCap     int
	// 同步时机：nil 表示保持现值；0 是合法取值（如每日 0 点触发），必须用指针区分。
	DailySyncHour     *int
	DailySyncMinute   *int
	WeeklySyncWeekday *int
	WeeklySyncHour    *int
	WeeklySyncMinute  *int
	MonthlySyncDay    *int
	MonthlySyncHour   *int
	MonthlySyncMinute *int
}

func (service *AdminService) SaveSettings(ctx context.Context, workspaceID, actorID uint, input SettingsInput) (CollectionConfig, error) {
	existing, err := service.GetSettings(ctx, workspaceID)
	if err != nil {
		return CollectionConfig{}, err
	}
	timezone := strings.TrimSpace(input.Timezone)
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return CollectionConfig{}, fmt.Errorf("%w: 时区无效", ErrAdminInvalid)
		}
		existing.Timezone = timezone
	}
	if input.PageCap != 0 {
		if input.PageCap < 1 || input.PageCap > 500 {
			return CollectionConfig{}, fmt.Errorf("%w: 单次页数上限须在 1-500 之间", ErrAdminInvalid)
		}
		existing.PageCap = input.PageCap
	}
	if input.DailySyncHour != nil {
		if *input.DailySyncHour < 0 || *input.DailySyncHour > 23 {
			return CollectionConfig{}, fmt.Errorf("%w: 每日同步时刻须在 0-23 点之间", ErrAdminInvalid)
		}
		existing.DailySyncHour = *input.DailySyncHour
	}
	if input.DailySyncMinute != nil {
		if *input.DailySyncMinute < 0 || *input.DailySyncMinute > 59 {
			return CollectionConfig{}, fmt.Errorf("%w: 每日同步分钟须在 0-59 之间", ErrAdminInvalid)
		}
		existing.DailySyncMinute = *input.DailySyncMinute
	}
	if input.WeeklySyncWeekday != nil {
		if *input.WeeklySyncWeekday < 0 || *input.WeeklySyncWeekday > 6 {
			return CollectionConfig{}, fmt.Errorf("%w: 每周同步日无效", ErrAdminInvalid)
		}
		existing.WeeklySyncWeekday = *input.WeeklySyncWeekday
	}
	if input.WeeklySyncHour != nil {
		if *input.WeeklySyncHour < 0 || *input.WeeklySyncHour > 23 {
			return CollectionConfig{}, fmt.Errorf("%w: 每周同步时刻须在 0-23 点之间", ErrAdminInvalid)
		}
		existing.WeeklySyncHour = *input.WeeklySyncHour
	}
	if input.WeeklySyncMinute != nil {
		if *input.WeeklySyncMinute < 0 || *input.WeeklySyncMinute > 59 {
			return CollectionConfig{}, fmt.Errorf("%w: 每周同步分钟须在 0-59 之间", ErrAdminInvalid)
		}
		existing.WeeklySyncMinute = *input.WeeklySyncMinute
	}
	if input.MonthlySyncDay != nil {
		// 上限 28：二月没有 29/30/31 号，避免触发日永远跳过。
		if *input.MonthlySyncDay < 1 || *input.MonthlySyncDay > 28 {
			return CollectionConfig{}, fmt.Errorf("%w: 每月同步日须在 1-28 号之间", ErrAdminInvalid)
		}
		existing.MonthlySyncDay = *input.MonthlySyncDay
	}
	if input.MonthlySyncHour != nil {
		if *input.MonthlySyncHour < 0 || *input.MonthlySyncHour > 23 {
			return CollectionConfig{}, fmt.Errorf("%w: 每月同步时刻须在 0-23 点之间", ErrAdminInvalid)
		}
		existing.MonthlySyncHour = *input.MonthlySyncHour
	}
	if input.MonthlySyncMinute != nil {
		if *input.MonthlySyncMinute < 0 || *input.MonthlySyncMinute > 59 {
			return CollectionConfig{}, fmt.Errorf("%w: 每月同步分钟须在 0-59 之间", ErrAdminInvalid)
		}
		existing.MonthlySyncMinute = *input.MonthlySyncMinute
	}
	if input.BotPatterns != nil {
		patterns := make([]string, 0, len(input.BotPatterns))
		for _, pattern := range input.BotPatterns {
			if trimmed := strings.TrimSpace(pattern); trimmed != "" {
				patterns = append(patterns, trimmed)
			}
		}
		existing.BotPatterns = patterns
	}
	existing.Version++
	existing.UpdatedBy = actorID
	return service.deps.Settings.SaveSettings(ctx, workspaceID, existing)
}

func (service *AdminService) isMember(ctx context.Context, workspaceID, userID uint) bool {
	members, err := service.deps.Members.FindActiveMemberCandidates(ctx, workspaceID)
	if err != nil {
		return false
	}
	for _, member := range members {
		if member.FactID == userID {
			return true
		}
	}
	return false
}

func (service *AdminService) reaggregate(ctx context.Context, workspaceID uint) {
	if service.deps.Reaggregator == nil {
		return
	}
	_ = service.deps.Reaggregator.ReaggregateWorkspace(ctx, workspaceID)
}

func validateConnectionInput(input ConnectionInput, creating bool) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%w: 连接名称不能为空", ErrAdminInvalid)
	}
	if input.Provider != ProviderGitHub && input.Provider != ProviderGitLab {
		return fmt.Errorf("%w: 不支持的数据源类型", ErrAdminInvalid)
	}
	if creating && strings.TrimSpace(input.Token) == "" {
		return fmt.Errorf("%w: 访问令牌不能为空", ErrAdminInvalid)
	}
	return nil
}

func enabledOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func derefFilterRules(rules *FilterRules) FilterRules {
	if rules == nil {
		return FilterRules{}
	}
	return *rules
}

func defaultCollectionConfig() CollectionConfig {
	// BotPatterns 是不可配置的内置黑名单：命中关键字的提交只算仓库活动，
	// 不计入成员个人贡献。列表与用户数据中的 AI/机器人作者保持同步。
	return CollectionConfig{
		Timezone: "Asia/Shanghai",
		CoauthorPolicy: "primary",
		BotPatterns:    []string{"[bot]", "noreply@", "cursoragent@"},
		PageCap:        40,
		DailySyncHour: 9, DailySyncMinute: 0,
		WeeklySyncWeekday: int(time.Monday), WeeklySyncHour: 9, WeeklySyncMinute: 0,
		MonthlySyncDay: 1, MonthlySyncHour: 9, MonthlySyncMinute: 0,
		Version: 1,
	}
}

func mapAdminError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrAdminNotFound), errors.Is(err, ErrAdminInvalid), errors.Is(err, ErrAdminUnsupported):
		return err
	case strings.Contains(err.Error(), "not found"):
		return ErrAdminNotFound
	default:
		return err
	}
}
