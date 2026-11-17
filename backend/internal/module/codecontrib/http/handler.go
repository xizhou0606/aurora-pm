// Package codecontribhttp adapts the SCM contribution admin use cases to the
// authenticated PM HTTP contract. It deliberately receives no database handle
// and never re-implements RBAC — route authorization is enforced by the
// access catalog middleware.
package codecontribhttp

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	codecontrib "github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

// Handler mounts the connection/repository/binding/settings endpoints below an
// already authenticated and workspace-scoped platform group.
type Handler struct {
	admin *codecontrib.AdminService
}

func NewHandler(admin *codecontrib.AdminService) (*Handler, error) {
	if admin == nil {
		return nil, codecontrib.ErrAdminInvalid
	}
	return &Handler{admin: admin}, nil
}

// RegisterRoutes mounts the admin configuration surface.
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	scm := group.Group("/scm")
	scm.GET("/connections", handler.listConnections)
	scm.POST("/connections", handler.createConnection)
	scm.PUT("/connections/:id", handler.updateConnection)
	scm.DELETE("/connections/:id", handler.deleteConnection)
	scm.POST("/connections/:id/test", handler.testConnection)
	scm.GET("/connections/:id/repos", handler.listAccessibleRepos)
	scm.GET("/repositories", handler.listRepositories)
	scm.POST("/repositories", handler.createRepository)
	scm.PUT("/repositories/:id", handler.updateRepository)
	scm.DELETE("/repositories/:id", handler.deleteRepository)
	scm.GET("/bindings", handler.listBindings)
	scm.POST("/bindings/:id/confirm", handler.confirmBinding)
	scm.POST("/bindings/:id/reject", handler.rejectBinding)
	scm.GET("/settings", handler.getSettings)
	scm.PUT("/settings", handler.saveSettings)
}

// connectionView is the admin projection of a connection: the token never
// leaves the server, only its masked suffix does.
type connectionView struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	BaseURL     string `json:"base_url,omitempty"`
	Enabled     bool   `json:"enabled"`
	TokenMasked string `json:"token_masked"`
	Version     uint64 `json:"version"`
	UpdatedBy   uint   `json:"updated_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func connectionViewOf(connection codecontrib.Connection) connectionView {
	return connectionView{
		ID: connection.ID, Name: connection.Name, Provider: string(connection.Provider),
		BaseURL: connection.BaseURL, Enabled: connection.Enabled,
		TokenMasked: connection.MaskedToken(), Version: connection.Version,
		UpdatedBy: connection.UpdatedBy,
		CreatedAt: connection.CreatedAt.Format(time.RFC3339),
		UpdatedAt: connection.UpdatedAt.Format(time.RFC3339),
	}
}

type connectionPayload struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Token    string `json:"token"`
	Enabled  *bool  `json:"enabled"`
}

func (handler *Handler) listConnections(c *gin.Context) {
	workspaceID, ok := scmWorkspaceID(c)
	if !ok {
		return
	}
	connections, err := handler.admin.ListConnections(c.Request.Context(), workspaceID)
	if err != nil {
		scmError(c, err)
		return
	}
	views := make([]connectionView, 0, len(connections))
	for _, connection := range connections {
		views = append(views, connectionViewOf(connection))
	}
	scmOK(c, views)
}

func (handler *Handler) createConnection(c *gin.Context) {
	workspaceID, principal, ok := scmContext(c)
	if !ok {
		return
	}
	var payload connectionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		scmBad(c, err.Error())
		return
	}
	connection, err := handler.admin.CreateConnection(c.Request.Context(), workspaceID, principal.UserID,
		codecontrib.ConnectionInput{
			Name: payload.Name, Provider: codecontrib.Provider(payload.Provider),
			BaseURL: payload.BaseURL, Token: payload.Token, Enabled: payload.Enabled,
		})
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, connectionViewOf(connection))
}

func (handler *Handler) updateConnection(c *gin.Context) {
	workspaceID, principal, ok := scmContext(c)
	if !ok {
		return
	}
	id, ok := scmPathID(c)
	if !ok {
		return
	}
	var payload connectionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		scmBad(c, err.Error())
		return
	}
	connection, err := handler.admin.UpdateConnection(c.Request.Context(), workspaceID, id, principal.UserID,
		codecontrib.ConnectionInput{
			Name: payload.Name, Provider: codecontrib.Provider(payload.Provider),
			BaseURL: payload.BaseURL, Token: payload.Token, Enabled: payload.Enabled,
		})
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, connectionViewOf(connection))
}

func (handler *Handler) deleteConnection(c *gin.Context) {
	workspaceID, ok := scmWorkspaceID(c)
	if !ok {
		return
	}
	id, ok := scmPathID(c)
	if !ok {
		return
	}
	if err := handler.admin.DeleteConnection(c.Request.Context(), workspaceID, id); err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, gin.H{"deleted": true})
}

func (handler *Handler) testConnection(c *gin.Context) {
	workspaceID, ok := scmWorkspaceID(c)
	if !ok {
		return
	}
	id, ok := scmPathID(c)
	if !ok {
		return
	}
	account, err := handler.admin.TestConnection(c.Request.Context(), workspaceID, id)
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, account)
}

func (handler *Handler) listAccessibleRepos(c *gin.Context) {
	workspaceID, ok := scmWorkspaceID(c)
	if !ok {
		return
	}
	id, ok := scmPathID(c)
	if !ok {
		return
	}
	listing, err := handler.admin.ListAccessibleRepos(c.Request.Context(), workspaceID, id, c.Query("cursor"))
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, listing)
}

type repositoryPayload struct {
	ConnectionID  uint                     `json:"connection_id"`
	ExternalID    string                   `json:"external_id"`
	Path          string                   `json:"path"`
	DefaultBranch string                   `json:"default_branch"`
	FilterRules   *codecontrib.FilterRules `json:"filter_rules"`
	ProjectSetID  *uint                    `json:"project_set_id"`
	Enabled       *bool                    `json:"enabled"`
}

func (handler *Handler) listRepositories(c *gin.Context) {
	workspaceID, ok := scmWorkspaceID(c)
	if !ok {
		return
	}
	repositories, err := handler.admin.ListRepositories(c.Request.Context(), workspaceID)
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, repositories)
}

func (handler *Handler) createRepository(c *gin.Context) {
	workspaceID, principal, ok := scmContext(c)
	if !ok {
		return
	}
	var payload repositoryPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		scmBad(c, err.Error())
		return
	}
	repository, err := handler.admin.CreateRepository(c.Request.Context(), workspaceID, principal.UserID,
		codecontrib.RepositoryInput{
			ConnectionID: payload.ConnectionID, ExternalID: payload.ExternalID,
			Path: payload.Path, DefaultBranch: payload.DefaultBranch,
			FilterRules: payload.FilterRules, ProjectSetID: payload.ProjectSetID,
			Enabled: payload.Enabled,
		})
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, repository)
}

func (handler *Handler) updateRepository(c *gin.Context) {
	workspaceID, principal, ok := scmContext(c)
	if !ok {
		return
	}
	id, ok := scmPathID(c)
	if !ok {
		return
	}
	var payload repositoryPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		scmBad(c, err.Error())
		return
	}
	repository, err := handler.admin.UpdateRepository(c.Request.Context(), workspaceID, id, principal.UserID,
		codecontrib.RepositoryInput{
			ConnectionID: payload.ConnectionID, ExternalID: payload.ExternalID,
			Path: payload.Path, DefaultBranch: payload.DefaultBranch,
			FilterRules: payload.FilterRules, ProjectSetID: payload.ProjectSetID,
			Enabled: payload.Enabled,
		})
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, repository)
}

func (handler *Handler) deleteRepository(c *gin.Context) {
	workspaceID, ok := scmWorkspaceID(c)
	if !ok {
		return
	}
	id, ok := scmPathID(c)
	if !ok {
		return
	}
	if err := handler.admin.DeleteRepository(c.Request.Context(), workspaceID, id); err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, gin.H{"deleted": true})
}

func (handler *Handler) listBindings(c *gin.Context) {
	workspaceID, ok := scmWorkspaceID(c)
	if !ok {
		return
	}
	statuses := []codecontrib.BindingStatus{}
	for _, raw := range strings.Split(c.Query("status"), ",") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		statuses = append(statuses, codecontrib.BindingStatus(trimmed))
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	bindings, err := handler.admin.ListBindings(c.Request.Context(), workspaceID, statuses, limit)
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, bindings)
}

type bindingConfirmPayload struct {
	UserID uint `json:"user_id"`
}

func (handler *Handler) confirmBinding(c *gin.Context) {
	workspaceID, principal, ok := scmContext(c)
	if !ok {
		return
	}
	id, ok := scmPathID(c)
	if !ok {
		return
	}
	var payload bindingConfirmPayload
	if err := c.ShouldBindJSON(&payload); err != nil || payload.UserID == 0 {
		scmBad(c, "user_id is required")
		return
	}
	row, err := handler.admin.ConfirmBinding(c.Request.Context(), workspaceID, id, payload.UserID, principal.UserID)
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, row)
}

func (handler *Handler) rejectBinding(c *gin.Context) {
	workspaceID, principal, ok := scmContext(c)
	if !ok {
		return
	}
	id, ok := scmPathID(c)
	if !ok {
		return
	}
	row, err := handler.admin.RejectBinding(c.Request.Context(), workspaceID, id, principal.UserID)
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, row)
}

func (handler *Handler) getSettings(c *gin.Context) {
	workspaceID, ok := scmWorkspaceID(c)
	if !ok {
		return
	}
	config, err := handler.admin.GetSettings(c.Request.Context(), workspaceID)
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, config)
}

type settingsPayload struct {
	Timezone          string   `json:"timezone"`
	BotPatterns       []string `json:"bot_patterns"`
	PageCap           int      `json:"page_cap"`
	DailySyncHour     *int     `json:"daily_sync_hour"`
	DailySyncMinute   *int     `json:"daily_sync_minute"`
	WeeklySyncWeekday *int     `json:"weekly_sync_weekday"`
	WeeklySyncHour    *int     `json:"weekly_sync_hour"`
	WeeklySyncMinute  *int     `json:"weekly_sync_minute"`
	MonthlySyncDay    *int     `json:"monthly_sync_day"`
	MonthlySyncHour   *int     `json:"monthly_sync_hour"`
	MonthlySyncMinute *int     `json:"monthly_sync_minute"`
}

func (handler *Handler) saveSettings(c *gin.Context) {
	workspaceID, principal, ok := scmContext(c)
	if !ok {
		return
	}
	var payload settingsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		scmBad(c, err.Error())
		return
	}
	config, err := handler.admin.SaveSettings(c.Request.Context(), workspaceID, principal.UserID,
		codecontrib.SettingsInput{
			Timezone: payload.Timezone, BotPatterns: payload.BotPatterns,
			PageCap: payload.PageCap,
			DailySyncHour: payload.DailySyncHour, DailySyncMinute: payload.DailySyncMinute,
			WeeklySyncWeekday: payload.WeeklySyncWeekday, WeeklySyncHour: payload.WeeklySyncHour,
			WeeklySyncMinute: payload.WeeklySyncMinute,
			MonthlySyncDay:   payload.MonthlySyncDay, MonthlySyncHour: payload.MonthlySyncHour,
			MonthlySyncMinute: payload.MonthlySyncMinute,
		})
	if err != nil {
		scmError(c, err)
		return
	}
	scmOK(c, config)
}

func scmWorkspaceID(c *gin.Context) (uint, bool) {
	value, found := c.Get(pmauth.WorkspaceIDContextKey)
	if !found {
		scmFailure(c, http.StatusUnauthorized, "SCM_WORKSPACE_REQUIRED", "workspace context is required")
		return 0, false
	}
	workspaceID, ok := value.(uint)
	if !ok || workspaceID == 0 {
		scmFailure(c, http.StatusUnauthorized, "SCM_WORKSPACE_REQUIRED", "workspace context is required")
		return 0, false
	}
	return workspaceID, true
}

func scmContext(c *gin.Context) (uint, pmauth.Principal, bool) {
	workspaceID, ok := scmWorkspaceID(c)
	if !ok {
		return 0, pmauth.Principal{}, false
	}
	value, found := c.Get(pmauth.PrincipalContextKey)
	if !found {
		scmFailure(c, http.StatusUnauthorized, "SCM_SESSION_REQUIRED", "authentication is required")
		return 0, pmauth.Principal{}, false
	}
	principal, ok := value.(pmauth.Principal)
	if !ok || principal.UserID == 0 {
		scmFailure(c, http.StatusUnauthorized, "SCM_SESSION_REQUIRED", "authentication is required")
		return 0, pmauth.Principal{}, false
	}
	return workspaceID, principal, true
}

func scmPathID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || value == 0 {
		scmBad(c, "id is invalid")
		return 0, false
	}
	return uint(value), true
}

func scmOK(c *gin.Context, value any) {
	c.JSON(http.StatusOK, gin.H{"data": value})
}

func scmBad(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": message})
}

func scmFailure(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "error": message})
}

func scmError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, codecontrib.ErrAdminInvalid):
		scmBad(c, err.Error())
	case errors.Is(err, codecontrib.ErrAdminNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "record not found"})
	case errors.Is(err, codecontrib.ErrAdminUnsupported):
		c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{"error": "provider not configured"})
	default:
		// 不向客户端泄漏内部细节，但服务端必须留下可排查的错误现场。
		log.Printf("codecontrib handler: %s %s failed: %v", c.Request.Method, c.Request.URL.Path, err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
