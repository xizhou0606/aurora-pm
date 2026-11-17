package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/http"
)

const (
	versionReportListRoute   = "/api/v1/platform/pm/testing/version-reports"
	versionReportUploadRoute = "/api/v1/platform/pm/testing/version-reports/upload"
	versionReportLinkRoute   = "/api/v1/platform/pm/testing/version-reports/link"
)

func (s *Server) listWorkflowTestLibrary(c *gin.Context) {
	kind := strings.TrimSpace(c.Query("kind"))
	if kind != "test_case" && kind != "test_report" {
		bad(c, "kind must be test_case or test_report")
		return
	}
	if kind == "test_report" && !s.requireAccessRoute(c, http.MethodGet, versionReportListRoute) {
		return
	}
	s.listTestingLibraryKind(c, kind)
}

func (s *Server) listVersionTestReports(c *gin.Context) {
	s.listTestingLibraryKind(c, "test_report")
}

func (s *Server) listTestingLibraryKind(c *gin.Context, kind string) {
	workspace, ok := currentWorkspace(c)
	if !ok {
		return
	}
	filterQuery := strings.TrimSpace(c.Query("q"))
	projectID := queryUint(c, "project_id")
	planUnitID := queryUint(c, "plan_unit_id")
	uploaders, ok := s.libraryUploaders(c, nil)
	if !ok {
		return
	}
	// Testing-library rows are project data (requirement test cases hang off
	// requirements.project_new_id; test reports off reports.project_set_id).
	// A workspace member without access to a project must not see, search, or
	// count its rows, so the access filter is applied inside both listing
	// queries rather than after fetching.
	testCaseScope, testCaseScopeOK := s.projectAccessScope(c, "requirements", "project_new_id")
	reportScope, reportScopeOK := s.projectAccessScope(c, "plan_unit_test_reports", "project_set_id")
	if !testCaseScopeOK || !reportScopeOK {
		c.JSON(http.StatusOK, gin.H{"data": make([]gin.H, 0), "total": 0})
		return
	}
	data := make([]gin.H, 0)
	if kind == "test_case" {
		rows, err := s.materials.ListTestCases(workspace.ID, requirement.LibraryFilter{
			Query: filterQuery, ProjectID: projectID, PlanUnitID: planUnitID,
			AccessScope: requirement.LibraryAccessScope{SQL: testCaseScope.SQL, Args: testCaseScope.Args},
		})
		if err != nil {
			dbFail(c)
			return
		}
		uploaderIDs := make([]uint, 0, len(rows))
		for _, row := range rows {
			uploaderIDs = append(uploaderIDs, row.UploaderUserID)
		}
		uploaders, ok = s.libraryUploaders(c, uploaderIDs)
		if !ok {
			return
		}
		for _, row := range rows {
			source := "file"
			url := requirementhttp.MaterialDTO(row.Material)["url"]
			if requirementhttp.IsMaterialLink(row.Material) {
				source = "link"
				url = row.Material.URL
			}
			item := gin.H{
				"id": row.ID, "kind": "test_case", "name": row.Name, "content_type": row.ContentType,
				"size": row.Size, "url": url, "source": source,
				"created_at": row.CreatedAt, "instance_id": 0, "node_id": "",
				"subject_type": "requirement", "subject_id": row.RequirementID, "subject_title": row.RequirementTitle,
				"uploader_user_id": row.UploaderUserID,
			}
			attachLibraryUploader(item, row.UploaderUserID, uploaders)
			if row.ProjectSetID > 0 {
				item["project_id"] = row.ProjectSetID
				item["project_name"] = row.ProjectName
			}
			if row.PlanUnitID > 0 {
				item["plan_unit_id"] = row.PlanUnitID
				item["plan_unit_title"] = row.PlanUnitTitle
			}
			data = append(data, item)
		}
		c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
		return
	}
	rows, err := s.reports.List(workspace.ID, quality.TestReportFilter{
		Query: filterQuery, ProjectID: projectID, PlanUnitID: planUnitID,
		AccessScope: quality.TestReportAccessScope{SQL: reportScope.SQL, Args: reportScope.Args},
	})
	if err != nil {
		dbFail(c)
		return
	}
	uploaderIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		uploaderIDs = append(uploaderIDs, row.UploaderUserID)
	}
	uploaders, ok = s.libraryUploaders(c, uploaderIDs)
	if !ok {
		return
	}
	for _, row := range rows {
		source := "file"
		url := fmt.Sprintf("/api/v1/platform/pm/testing/reports/%d/download", row.ID)
		if row.Kind == quality.TestReportKindLink {
			source = "link"
			url = row.URL
		}
		item := gin.H{
			"id": row.ID, "kind": "test_report", "name": row.Name, "content_type": row.ContentType,
			"size": row.Size, "url": url, "source": source, "created_at": row.CreatedAt,
			"instance_id": 0, "node_id": "", "subject_type": "plan_unit", "subject_id": row.PlanUnitID,
			"subject_title": "", "uploader_user_id": row.UploaderUserID,
		}
		attachLibraryUploader(item, row.UploaderUserID, uploaders)
		if row.ProjectSetID > 0 {
			item["project_id"] = row.ProjectSetID
			item["project_name"] = row.ProjectName
		}
		if row.PlanUnitID > 0 {
			item["plan_unit_id"] = row.PlanUnitID
			item["plan_unit_title"] = row.PlanUnitTitle
		}
		data = append(data, item)
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

func (s *Server) createTestingLibraryItem(c *gin.Context) {
	kind := strings.TrimSpace(c.PostForm("kind"))
	if kind == "" {
		kind = strings.TrimSpace(c.Query("kind"))
	}
	if kind == "test_report" {
		route := versionReportUploadRoute
		if strings.TrimSpace(c.PostForm("url")) != "" {
			route = versionReportLinkRoute
		}
		if !s.requireAccessRoute(c, http.MethodPost, route) {
			return
		}
	}
	s.createTestingLibraryKind(c, kind, "auto")
}

func (s *Server) uploadVersionTestReport(c *gin.Context) {
	s.createTestingLibraryKind(c, "test_report", "file")
}

func (s *Server) linkVersionTestReport(c *gin.Context) {
	s.createTestingLibraryKind(c, "test_report", "link")
}

func (s *Server) createTestingLibraryKind(c *gin.Context, kind, reportMode string) {
	workspace, ok := currentWorkspace(c)
	if !ok {
		return
	}
	switch kind {
	case "test_case":
		requirementID := queryUint(c, "requirement_id")
		if requirementID == 0 {
			requirementID = formUint(c, "requirement_id")
		}
		if requirementID == 0 {
			bad(c, "requirement_id is required")
			return
		}
		requirementRow, project, allowed := s.requirementByID(c, requirementID)
		if !allowed {
			return
		}
		row, stored := s.storeRequirementMaterialFile(
			c, requirementRow, project.ID, requirement.MaterialKindTestCase,
			requirement.MaterialSourceTestingLibrary, currentFactUserID(c), nil,
		)
		if !stored {
			return
		}
		created(c, requirementhttp.MaterialDTO(row))
	case "test_report":
		planUnitID := formUint(c, "plan_unit_id")
		if planUnitID == 0 {
			planUnitID = queryUint(c, "plan_unit_id")
		}
		if planUnitID == 0 {
			bad(c, "plan_unit_id is required")
			return
		}
		var unit projectgorm.PlanUnit
		if s.db.Where("id = ? AND workspace_id = ?", planUnitID, workspace.ID).First(&unit).Error != nil {
			fail(c, http.StatusNotFound, "plan unit not found")
			return
		}
		if !s.requireProjectAccess(c, workspace.ID, unit.ProjectSetID) {
			return
		}
		link := strings.TrimSpace(c.PostForm("url"))
		if reportMode == "file" && link != "" {
			bad(c, "test report upload does not accept url")
			return
		}
		if reportMode == "link" && link == "" {
			bad(c, "test report url is required")
			return
		}
		if link != "" {
			if !quality.ValidReportURL(link) {
				bad(c, "test report url is invalid")
				return
			}
			name := strings.TrimSpace(c.PostForm("name"))
			if name == "" {
				name = link
			}
			row, err := s.reports.Create(quality.TestReportInput{
				WorkspaceID: workspace.ID, PlanUnitID: unit.ID, ProjectSetID: unit.ProjectSetID,
				Kind: quality.TestReportKindLink, Name: safeMaterialFilename(name),
				ContentType: "text/uri-list", ObjectKey: fmt.Sprintf("link/%d/%d/%d", workspace.ID, unit.ID, time.Now().UTC().UnixNano()),
				URL: link, UploaderUserID: currentFactUserID(c),
			})
			if err != nil {
				dbFail(c)
				return
			}
			created(c, gin.H{"id": row.ID, "kind": "test_report", "name": row.Name, "url": row.URL, "source": "link"})
			return
		}
		s.uploadPlanUnitTestReportFile(c, unit)
	default:
		bad(c, "kind must be test_case or test_report")
	}
}

func (s *Server) uploadPlanUnitTestReportFile(c *gin.Context, unit projectgorm.PlanUnit) {
	// Reuse test-report file rules from the retired workflow uploader.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, workflowTestReportArtifactMaxBytes+1024*1024)
	if err := c.Request.ParseMultipartForm(8 * 1024 * 1024); err != nil {
		bad(c, "invalid multipart form")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil || fileHeader.Size <= 0 {
		bad(c, "test report file required")
		return
	}
	if fileHeader.Size > workflowTestReportArtifactMaxBytes {
		bad(c, "test report file too large (max 20MB)")
		return
	}
	if s.materialStore == nil || s.materialStoreErr != nil || !s.cfg.ObjectStorage.Enabled {
		fail(c, http.StatusServiceUnavailable, "system object storage is required for test report upload")
		return
	}
	name := safeWorkflowTestReportName(fileHeader.Filename)
	extension := strings.ToLower(strings.TrimPrefix(name[strings.LastIndex(name, "."):], ""))
	if !strings.Contains(name, ".") {
		bad(c, "test report file must be PDF, DOC, DOCX, or XLSX")
		return
	}
	extension = strings.ToLower(name[strings.LastIndex(name, "."):])
	contentType, validType := workflowTestReportContentType(extension)
	if !validType {
		bad(c, "test report file must be PDF, DOC, DOCX, or XLSX")
		return
	}
	source, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to open test report file")
		return
	}
	defer source.Close()
	unique, err := randomMaterialHex(16)
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to prepare test report upload")
		return
	}
	objectKey := buildRequirementDesignAssetKey(unit.WorkspaceID, unit.TeamID, unit.ProjectSetID, s.cfg.ObjectStorage, time.Now().UTC(), unique+"-report", extension)
	if err := s.materialStore.PutObject(c.Request.Context(), objectKey, source, fileHeader.Size, contentType); err != nil {
		failCode(c, http.StatusBadGateway, apiErrorObjectStorageUploadFailed, "test report upload failed")
		return
	}
	publicURL, valid := publicMaterialURL(s, objectKey)
	if !valid {
		fail(c, http.StatusInternalServerError, "failed to build test report file url")
		return
	}
	row, err := s.reports.Create(quality.TestReportInput{
		WorkspaceID: unit.WorkspaceID, PlanUnitID: unit.ID, ProjectSetID: unit.ProjectSetID,
		Kind: quality.TestReportKindFile, Name: name, ContentType: contentType, Size: fileHeader.Size,
		ObjectKey: objectKey, URL: publicURL, UploaderUserID: currentFactUserID(c),
	})
	if err != nil {
		dbFail(c)
		return
	}
	created(c, gin.H{
		"id": row.ID, "kind": "test_report", "name": row.Name, "content_type": row.ContentType,
		"size": row.Size, "url": fmt.Sprintf("/api/v1/platform/pm/testing/reports/%d/download", row.ID), "source": "file",
	})
}

func publicMaterialURL(s *Server, objectKey string) (string, bool) {
	return objectstore.PublicURL(s.cfg.ObjectStorage, objectKey)
}

func (s *Server) downloadPlanUnitTestReport(c *gin.Context) {
	workspace, ok := currentWorkspace(c)
	if !ok {
		return
	}
	reportID, ok := paramID(c, "id")
	if !ok {
		return
	}
	row, err := s.reports.Get(workspace.ID, reportID)
	if err == quality.ErrTestReportNotFound {
		fail(c, http.StatusNotFound, "test report not found")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	// A report in a project the caller cannot access is indistinguishable from
	// a missing one: the download gate mirrors the listing scope.
	if !s.requireProjectAccess(c, workspace.ID, row.ProjectSetID) {
		return
	}
	if row.Kind == quality.TestReportKindLink {
		c.Redirect(http.StatusFound, row.URL)
		return
	}
	store, ready := s.materialStore.(workflowArtifactObjectReader)
	if !ready || store == nil || s.materialStoreErr != nil || !s.cfg.ObjectStorage.Enabled {
		fail(c, http.StatusServiceUnavailable, "object storage is unavailable")
		return
	}
	reader, err := store.GetObject(c.Request.Context(), row.ObjectKey)
	if err != nil {
		fail(c, http.StatusNotFound, "test report object not found")
		return
	}
	defer reader.Close()
	c.Header("Content-Type", row.ContentType)
	c.Header("Content-Length", fmt.Sprint(row.Size))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", row.Name))
	_, _ = io.Copy(c.Writer, reader)
}

func (s *Server) libraryUploaders(c *gin.Context, ids []uint) (map[uint]identitygorm.User, bool) {
	uploaders := map[uint]identitygorm.User{}
	ids = uniqueUintIDs(ids)
	if len(ids) == 0 {
		return uploaders, true
	}
	var rows []identitygorm.User
	if s.db.Where("id IN ?", ids).Find(&rows).Error != nil {
		dbFail(c)
		return nil, false
	}
	for _, row := range rows {
		uploaders[factUserID(row)] = row
		uploaders[row.ID] = row
	}
	return uploaders, true
}

func attachLibraryUploader(item gin.H, uploaderID uint, uploaders map[uint]identitygorm.User) {
	user, ok := uploaders[uploaderID]
	if !ok {
		return
	}
	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = user.Username
	}
	item["uploader_name"] = name
	if avatar := strings.TrimSpace(user.AvatarURL); avatar != "" {
		item["uploader_avatar_url"] = avatar
	}
}

func formUint(c *gin.Context, name string) uint {
	return queryUintFrom(c.PostForm(name))
}

func queryUintFrom(raw string) uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	var value uint
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0
		}
		value = value*10 + uint(r-'0')
	}
	return value
}
