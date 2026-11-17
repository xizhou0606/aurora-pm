// Package reporthttp adapts the report use cases to the authenticated PM HTTP
// contract. It deliberately receives no database handle.
package reporthttp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	report "github.com/xizhou0606/aurora-pm/backend/internal/module/report"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

// Handler mounts the reporting endpoints below an already authenticated and
// workspace-scoped platform group. Route authorization is enforced by the
// access catalog middleware; this adapter never re-implements RBAC.
type Handler struct {
	reports *report.Service
}

func NewHandler(reports *report.Service) (*Handler, error) {
	if reports == nil {
		return nil, report.ErrReportUnavailable
	}
	return &Handler{reports: reports}, nil
}

// RegisterRoutes mounts the reporting read, generation, and export endpoints.
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	reports := group.Group("/reports")
	reports.GET("/members", handler.memberDirectory)
	reports.GET("/members/:userId/preview", handler.memberReportPreview)
	reports.POST("/members/:userId/generate", handler.memberReportGenerate)
	reports.GET("/groups/:groupId/preview", handler.groupReportPreview)
	reports.POST("/groups/:groupId/generate", handler.groupReportGenerate)
	reports.GET("/departments/:departmentId/preview", handler.departmentReportPreview)
	reports.POST("/departments/:departmentId/generate", handler.departmentReportGenerate)
	reports.GET("/runs/:runId", handler.memberReportRunStatus)
	reports.POST("/runs/:runId/feishu-doc", handler.exportReportRunFeishuDoc)
}

func (handler *Handler) memberDirectory(c *gin.Context) {
	workspaceID, ok := reportWorkspaceID(c)
	if !ok {
		return
	}
	view, err := handler.reports.MemberDirectory(c.Request.Context(), workspaceID)
	if err != nil {
		reportError(c, err)
		return
	}
	reportOK(c, view)
}

func (handler *Handler) memberReportPreview(c *gin.Context) {
	handler.handleReportPreview(c, report.TargetMember, "userId")
}

func (handler *Handler) groupReportPreview(c *gin.Context) {
	handler.handleReportPreview(c, report.TargetGroup, "groupId")
}

func (handler *Handler) departmentReportPreview(c *gin.Context) {
	handler.handleReportPreview(c, report.TargetDepartment, "departmentId")
}

func (handler *Handler) handleReportPreview(c *gin.Context, kind report.ReportTargetKind, param string) {
	workspaceID, ok := reportWorkspaceID(c)
	if !ok {
		return
	}
	targetID, ok := reportPathID(c, param)
	if !ok {
		return
	}
	periodType, ok := reportPeriodQuery(c)
	if !ok {
		return
	}
	var result report.Report
	var err error
	switch kind {
	case report.TargetGroup:
		result, err = handler.reports.GroupReport(
			c.Request.Context(), workspaceID, targetID, periodType,
			c.Query("date"), reportLocation(c),
		)
	case report.TargetDepartment:
		result, err = handler.reports.DepartmentReport(
			c.Request.Context(), workspaceID, targetID, periodType,
			c.Query("date"), reportLocation(c),
		)
	default:
		result, err = handler.reports.MemberReport(
			c.Request.Context(), workspaceID, targetID, periodType,
			c.Query("date"), reportLocation(c),
		)
	}
	if err != nil {
		reportError(c, err)
		return
	}
	// 预览随行该视图最近一次 run（定时生成没有前端回执）：完成态直接展示
	// 评语与正文，进行中交给前端接管轮询。无 run 属正常情况，字段省略。
	response := reportPreviewResponse{Report: result}
	if run, runErr := handler.reports.LatestReportRunForView(
		c.Request.Context(), workspaceID, kind, targetID, result.Period,
	); runErr == nil {
		response.LatestRun = &run
	}
	reportOK(c, response)
}

// reportPreviewResponse wraps the facts report with the view's latest run so
// the page can surface scheduled narratives without a manual generate.
type reportPreviewResponse struct {
	Report    report.Report       `json:"report"`
	LatestRun *ai.ReportRunResult `json:"latest_run,omitempty"`
}

// memberReportGenerate and groupReportGenerate queue an AI reporting run. The
// response carries the computed facts immediately; the narrative arrives
// through the run status endpoint once the built-in report skill completes.
func (handler *Handler) memberReportGenerate(c *gin.Context) {
	handler.handleReportGenerate(c, report.TargetMember, "userId")
}

func (handler *Handler) groupReportGenerate(c *gin.Context) {
	handler.handleReportGenerate(c, report.TargetGroup, "groupId")
}

func (handler *Handler) departmentReportGenerate(c *gin.Context) {
	handler.handleReportGenerate(c, report.TargetDepartment, "departmentId")
}

func (handler *Handler) handleReportGenerate(c *gin.Context, kind report.ReportTargetKind, param string) {
	workspaceID, ok := reportWorkspaceID(c)
	if !ok {
		return
	}
	principal, ok := reportPrincipal(c)
	if !ok {
		return
	}
	targetID, ok := reportPathID(c, param)
	if !ok {
		return
	}
	var request reportGenerateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		reportBad(c, "invalid body")
		return
	}
	periodType, ok := reportPeriodValue(request.Type)
	if !ok {
		reportBad(c, "type must be one of daily, weekly, monthly")
		return
	}
	result, err := handler.reports.StartReport(
		c.Request.Context(), workspaceID, principal.UserID, kind, targetID, periodType,
		request.Date, reportLocation(c),
	)
	if err != nil {
		reportError(c, err)
		return
	}
	reportOK(c, result)
}

func (handler *Handler) memberReportRunStatus(c *gin.Context) {
	workspaceID, ok := reportWorkspaceID(c)
	if !ok {
		return
	}
	runID, ok := reportPathID(c, "runId")
	if !ok {
		return
	}
	result, err := handler.reports.MemberReportRunStatus(c.Request.Context(), workspaceID, runID)
	if err != nil {
		reportError(c, err)
		return
	}
	reportOK(c, result)
}

// exportReportRunFeishuDoc publishes a completed reporting run as a Feishu
// document and returns the created document reference. runId=0 reuses the
// same route (same publish action in the access catalog) to publish the
// current facts-only report, so uploading never requires a completed AI run.
func (handler *Handler) exportReportRunFeishuDoc(c *gin.Context) {
	workspaceID, ok := reportWorkspaceID(c)
	if !ok {
		return
	}
	runIDValue, err := strconv.ParseUint(strings.TrimSpace(c.Param("runId")), 10, 64)
	if err != nil {
		reportBad(c, "runId is invalid")
		return
	}
	// runId=0 selects the facts-only export on the same publish route.
	if runIDValue == 0 {
		handler.exportReportFacts(c)
		return
	}
	document, err := handler.reports.ExportReportRun(c.Request.Context(), workspaceID, uint(runIDValue))
	if err != nil {
		reportError(c, err)
		return
	}
	reportOK(c, document)
}

// reportExportFactsRequest carries the reporting context for the facts-only
// export; the workspace-scoped directory lookups inside the service remain
// the authorization boundary.
type reportExportFactsRequest struct {
	TargetKind string `json:"target_kind"`
	TargetID   uint   `json:"target_id"`
	Type       string `json:"type"`
	Date       string `json:"date"`
}

func (handler *Handler) exportReportFacts(c *gin.Context) {
	workspaceID, ok := reportWorkspaceID(c)
	if !ok {
		return
	}
	var request reportExportFactsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		reportBad(c, "invalid body")
		return
	}
	periodType, ok := reportPeriodValue(request.Type)
	if !ok {
		reportBad(c, "type must be one of daily, weekly, monthly")
		return
	}
	kind := report.TargetMember
	switch request.TargetKind {
	case "group":
		kind = report.TargetGroup
	case "department":
		kind = report.TargetDepartment
	}
	document, err := handler.reports.ExportReportFacts(
		c.Request.Context(), workspaceID, kind, request.TargetID, periodType,
		request.Date, reportLocation(c),
	)
	if err != nil {
		reportError(c, err)
		return
	}
	reportOK(c, document)
}

type reportGenerateRequest struct {
	Type string `json:"type"`
	Date string `json:"date"`
}

func reportWorkspaceID(c *gin.Context) (uint, bool) {
	value, found := c.Get(pmauth.WorkspaceIDContextKey)
	if !found {
		reportFailure(c, http.StatusUnauthorized, "REPORT_WORKSPACE_REQUIRED", "workspace context is required")
		return 0, false
	}
	workspaceID, ok := value.(uint)
	if !ok || workspaceID == 0 {
		reportFailure(c, http.StatusUnauthorized, "REPORT_WORKSPACE_REQUIRED", "workspace context is required")
		return 0, false
	}
	return workspaceID, true
}

func reportPrincipal(c *gin.Context) (pmauth.Principal, bool) {
	value, found := c.Get(pmauth.PrincipalContextKey)
	if !found {
		reportFailure(c, http.StatusUnauthorized, "REPORT_SESSION_REQUIRED", "authentication is required")
		return pmauth.Principal{}, false
	}
	principal, ok := value.(pmauth.Principal)
	if !ok || principal.UserID == 0 {
		reportFailure(c, http.StatusUnauthorized, "REPORT_SESSION_REQUIRED", "authentication is required")
		return pmauth.Principal{}, false
	}
	return principal, true
}

func reportPathID(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || value == 0 {
		reportBad(c, name+" is invalid")
		return 0, false
	}
	return uint(value), true
}

func reportPeriodQuery(c *gin.Context) (report.PeriodType, bool) {
	period, ok := reportPeriodValue(c.Query("type"))
	if !ok {
		reportBad(c, "type must be one of daily, weekly, monthly")
		return "", false
	}
	return period, true
}

func reportPeriodValue(raw string) (report.PeriodType, bool) {
	period := report.PeriodType(strings.ToLower(strings.TrimSpace(raw)))
	if !period.Valid() {
		return "", false
	}
	return period, true
}

// reportLocation mirrors the BI adapter: business day boundaries follow the
// viewer timezone from X-Timezone and fall back to UTC.
func reportLocation(c *gin.Context) *time.Location {
	value := strings.TrimSpace(c.GetHeader("X-Timezone"))
	if value == "" {
		return time.UTC
	}
	location, err := time.LoadLocation(value)
	if err != nil {
		return time.UTC
	}
	return location
}

func reportOK(c *gin.Context, value any) {
	c.JSON(http.StatusOK, gin.H{"data": value})
}

func reportBad(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": message})
}

func reportFailure(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "error": message})
}

func reportError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, report.ErrReportQueryInvalid), errors.Is(err, report.ErrReportPeriodInvalid):
		reportBad(c, err.Error())
	case errors.Is(err, report.ErrReportMemberNotFound), errors.Is(err, identity.ErrReportingDirectoryMemberNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "report member not found"})
	case errors.Is(err, ai.ErrReportRunNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "report run not found"})
	case errors.Is(err, report.ErrReportRunNotCompleted):
		reportFailure(c, http.StatusConflict, "REPORT_RUN_NOT_COMPLETED",
			"the reporting run has not completed yet")
	case errors.Is(err, report.ErrReportExportUnavailable), errors.Is(err, ai.ErrReportRunUnavailable):
		reportFailure(c, http.StatusConflict, "REPORT_EXPORT_UNAVAILABLE",
			"report export is unavailable; enable the AI or Feishu capability first")
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "report operation failed"})
	}
}
