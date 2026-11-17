package qualitygorm

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
)

type TestReportRepository struct {
	db *gorm.DB
}

func NewTestReportRepository(db *gorm.DB) *TestReportRepository {
	return &TestReportRepository{db: db}
}

func (repository *TestReportRepository) Create(input quality.TestReportInput) (quality.TestReport, error) {
	row := PlanUnitTestReport{
		WorkspaceID: input.WorkspaceID, PlanUnitID: input.PlanUnitID, ProjectSetID: input.ProjectSetID,
		Kind: input.Kind, Name: input.Name, ContentType: input.ContentType, Size: input.Size,
		ObjectKey: input.ObjectKey, URL: input.URL, UploaderUserID: input.UploaderUserID,
	}
	if err := repository.db.Create(&row).Error; err != nil {
		return quality.TestReport{}, err
	}
	return testReportFromRow(row, "", ""), nil
}

func (repository *TestReportRepository) List(workspaceID uint, filter quality.TestReportFilter) ([]quality.TestReport, error) {
	query := repository.db.Table("plan_unit_test_reports AS reports").
		Select("reports.*, plan_units.title AS plan_unit_title, project_sets.name AS project_name").
		Joins("JOIN plan_units ON plan_units.id = reports.plan_unit_id AND plan_units.workspace_id = reports.workspace_id").
		Joins("LEFT JOIN project_sets ON project_sets.id = reports.project_set_id AND project_sets.workspace_id = reports.workspace_id").
		Where("reports.workspace_id = ?", workspaceID)
	if filter.ProjectID > 0 {
		query = query.Where("reports.project_set_id = ?", filter.ProjectID)
	}
	if filter.PlanUnitID > 0 {
		query = query.Where("reports.plan_unit_id = ?", filter.PlanUnitID)
	}
	if needle := strings.TrimSpace(filter.Query); needle != "" {
		like := "%" + needle + "%"
		query = query.Where(
			"reports.name LIKE ? OR plan_units.title LIKE ? OR project_sets.name LIKE ?",
			like, like, like,
		)
	}
	if filter.AccessScope.SQL != "" {
		query = query.Where(filter.AccessScope.SQL, filter.AccessScope.Args...)
	}
	var rows []libraryReportRow
	if err := query.Order("reports.id DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]quality.TestReport, 0, len(rows))
	for _, row := range rows {
		result = append(result, testReportFromRow(row.PlanUnitTestReport, row.PlanUnitTitle, row.ProjectName))
	}
	return result, nil
}

func (repository *TestReportRepository) Get(workspaceID, reportID uint) (quality.TestReport, error) {
	var row PlanUnitTestReport
	err := repository.db.Where("id = ? AND workspace_id = ?", reportID, workspaceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return quality.TestReport{}, quality.ErrTestReportNotFound
	}
	if err != nil {
		return quality.TestReport{}, err
	}
	return testReportFromRow(row, "", ""), nil
}

type libraryReportRow struct {
	PlanUnitTestReport
	PlanUnitTitle string
	ProjectName   string
}

func testReportFromRow(row PlanUnitTestReport, planUnitTitle, projectName string) quality.TestReport {
	return quality.TestReport{
		ID: row.ID, WorkspaceID: row.WorkspaceID, PlanUnitID: row.PlanUnitID, ProjectSetID: row.ProjectSetID,
		Kind: row.Kind, Name: row.Name, ContentType: row.ContentType, Size: row.Size,
		ObjectKey: row.ObjectKey, URL: row.URL, UploaderUserID: row.UploaderUserID, CreatedAt: row.CreatedAt,
		PlanUnitTitle: planUnitTitle, ProjectName: projectName,
	}
}
