package httpapi

import (
	"errors"

	"github.com/gin-gonic/gin"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

type taskTestRunCountRow struct {
	TaskTestExecutionID uint
	Status              string
	Total               int64
}

func (s *Server) getTaskTestContext(c *gin.Context) {
	taskID, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, project, allowed := s.taskByID(c, taskID)
	if !allowed {
		return
	}
	var requirement requirementgorm.Requirement
	if err := s.db.Where(
		"id = ? AND workspace_id = ?", task.RequirementID, task.WorkspaceID,
	).First(&requirement).Error; err != nil {
		dbFail(c)
		return
	}
	var definition taskgorm.TaskTestDefinition
	definitionErr := s.db.Where(
		"workspace_id = ? AND work_task_id = ?", task.WorkspaceID, task.ID,
	).First(&definition).Error
	if definitionErr != nil && !errors.Is(definitionErr, gorm.ErrRecordNotFound) {
		dbFail(c)
		return
	}
	limit, offset := pagination(c, 50)
	query := s.db.Where("workspace_id = ? AND work_task_id = ?", task.WorkspaceID, task.ID)
	var total int64
	if query.Model(&taskgorm.TaskTestExecution{}).Count(&total).Error != nil {
		dbFail(c)
		return
	}
	var executions []taskgorm.TaskTestExecution
	if query.Order("id DESC").Limit(limit).Offset(offset).Find(&executions).Error != nil {
		dbFail(c)
		return
	}
	counts, err := s.taskTestRunCounts(task.WorkspaceID, executions)
	if err != nil {
		dbFail(c)
		return
	}
	executionRows := make([]gin.H, 0, len(executions))
	for _, execution := range executions {
		executionRows = append(executionRows, taskTestExecutionDTO(execution, counts[execution.ID]))
	}
	var definitionDTO any
	if definitionErr == nil {
		definitionDTO = taskTestDefinitionDTO(definition)
	}
	workspaceContext := s.loadWorkspaceContext(
		task.WorkspaceID,
		workspaceContextReferences{}.withTask(task),
	)
	hierarchyContext := s.loadWorkspaceTaskHierarchyContext(task.WorkspaceID, []taskgorm.WorkTask{task})
	taskContract := s.enrichWorkspaceTaskContract(
		taskDTO(task), task, workspaceContext, hierarchyContext,
	)
	ok(c, gin.H{
		"task":    taskContract,
		"project": gin.H{"id": project.ID, "name": project.Name, "code": project.Code},
		"requirement": gin.H{
			"id": requirement.ID, "title": requirement.Title,
			"priority": requirement.Priority, "status": requirement.Status,
			"acceptance_criteria": requirement.AcceptanceCriteria,
		},
		"definition": definitionDTO,
		"executions": gin.H{"items": executionRows, "total": total},
	})
}

func (s *Server) taskTestingTaskContract(task taskgorm.WorkTask) gin.H {
	workspaceContext := s.loadWorkspaceContext(
		task.WorkspaceID,
		workspaceContextReferences{}.withTask(task),
	)
	hierarchyContext := s.loadWorkspaceTaskHierarchyContext(task.WorkspaceID, []taskgorm.WorkTask{task})
	return s.enrichWorkspaceTaskContract(taskDTO(task), task, workspaceContext, hierarchyContext)
}

func (s *Server) taskTestRunCounts(
	workspaceID uint,
	executions []taskgorm.TaskTestExecution,
) (map[uint]map[string]int64, error) {
	result := make(map[uint]map[string]int64, len(executions))
	if len(executions) == 0 {
		return result, nil
	}
	ids := make([]uint, 0, len(executions))
	for _, execution := range executions {
		ids = append(ids, execution.ID)
		result[execution.ID] = map[string]int64{}
	}
	var rows []taskTestRunCountRow
	if err := s.db.Model(&taskgorm.TaskTestExecutionRun{}).
		Select("task_test_execution_id, status, COUNT(*) AS total").
		Where("workspace_id = ? AND task_test_execution_id IN ?", workspaceID, ids).
		Group("task_test_execution_id, status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.TaskTestExecutionID][row.Status] = row.Total
		result[row.TaskTestExecutionID]["total"] += row.Total
	}
	return result, nil
}
