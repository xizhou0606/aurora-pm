package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task/plan"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

var errWorkflowActionChanged = errors.New("workflow node is no longer active")
var errWorkflowAIStale = errors.New("workflow AI result is stale")
var errWorkflowAIRetryRequired = errors.New("workflow AI run must be retried")
var errTaskPlanSkipUnavailable = errors.New("requirement has no existing tasks to skip the task plan for")

const workflowAIStaleCode = "WORKFLOW_AI_STALE"

func (s *Server) finishWorkflowNode(c *gin.Context, instance workflowgorm.WorkflowInstance, definition workflowDefinition, node workflowNode, status, comment string, next []string, aiPathOverride string, skipTaskPlan bool, conditionDecision *bool, designURL string, nextAssignees map[string]uint, confirmRequirementLock bool) {
	now := time.Now().UTC()
	run := workflowgorm.WorkflowNodeRun{
		WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeID: node.ID,
		NodeType: node.Type, Status: status, StartedAt: now, CompletedAt: &now,
	}
	if !strings.HasPrefix(node.Type, "ai_") {
		targetNodeID := workflowSingleTransitionTarget(next)
		if conditionDecision != nil {
			run.OutputJSON = encodeWorkflowRunDecision(currentFactUserID(c), comment, *conditionDecision, designURL, targetNodeID)
		} else if designURL != "" {
			run.OutputJSON = encodeWorkflowRunMaterialTransition(currentFactUserID(c), comment, designURL, targetNodeID)
		} else {
			run.OutputJSON = encodeWorkflowRunComment(currentFactUserID(c), comment, targetNodeID)
		}
	}
	queuedAI := false
	expectedStatus := instance.Status
	expectedCurrent := instance.CurrentNodeIDs
	expectedPlans := instance.NodePlansJSON
	taskPlanProjectID := uint(0)
	if node.Capability == taskplan.Capability {
		taskPlanProjectID = s.workflowSubjectProjectID(s.db, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var taskPlanProject projectgorm.ProjectSet
		if node.Capability == taskplan.Capability {
			if taskPlanProjectID == 0 {
				return errTaskPlanApplyStale
			}
			var err error
			taskPlanProject, err = lockProjectSetTx(tx, instance.WorkspaceID, taskPlanProjectID)
			if err != nil {
				return fmt.Errorf("%w: project is unavailable", errTaskPlanApplyStale)
			}
		}
		claimed := tx.Model(&workflowgorm.WorkflowToken{}).
			Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", instance.WorkspaceID, instance.ID, node.ID, "active").
			Update("status", "processing")
		if claimed.Error != nil {
			return claimed.Error
		}
		if claimed.RowsAffected != 1 {
			return errWorkflowActionChanged
		}
		// The guard decision runs inside the action transaction so the
		// completion only commits when the requirement evidence still holds.
		if err := enforceWorkflowRequirementsGuardTx(tx, instance, node, status); err != nil {
			return err
		}
		if strings.HasPrefix(node.Type, "ai_") && status == "completed" {
			reusedAIResult := false
			var waiting workflowgorm.WorkflowNodeRun
			lookup := tx.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", instance.WorkspaceID, instance.ID, node.ID, "waiting").Order("id DESC").Limit(1).Find(&waiting)
			if lookup.Error != nil {
				return lookup.Error
			}
			var job workflowgorm.WorkflowJob
			var aiRun aigorm.AIRun
			if lookup.RowsAffected == 0 {
				sourceJob, previousAIRun, found, err := latestWorkflowAINodeExecutionTx(tx, instance, node.ID)
				if err != nil {
					return err
				}
				if !found {
					if aiPathOverride != "" {
						return errWorkflowAIOverrideUnavailable
					}
					run, err = s.queueWorkflowAINodeTx(tx, c, instance, node, now)
					if err != nil {
						return err
					}
					if err := tx.Model(&workflowgorm.WorkflowToken{}).
						Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", instance.WorkspaceID, instance.ID, node.ID, "processing").
						Update("status", "active").Error; err != nil {
						return err
					}
					queuedAI = true
					return nil
				}
				run, job, err = linkReusedWorkflowAIExecutionTx(tx, instance, node, sourceJob, now)
				if err != nil {
					return err
				}
				aiRun = previousAIRun
				reusedAIResult = true
			} else {
				run = waiting
				if err := tx.Where("workspace_id = ? AND instance_id = ? AND node_run_id = ?", instance.WorkspaceID, instance.ID, run.ID).First(&job).Error; err != nil {
					return err
				}
				var payload workflowAIJobPayload
				if json.Unmarshal([]byte(job.PayloadJSON), &payload) != nil || payload.AIRunID == 0 {
					return errors.New("workflow AI job payload is invalid")
				}
				if err := tx.Where("id = ? AND workspace_id = ?", payload.AIRunID, instance.WorkspaceID).First(&aiRun).Error; err != nil {
					return err
				}
			}
			var taskPlanRequirement requirementgorm.Requirement
			if node.Capability == taskplan.Capability {
				var err error
				taskPlanRequirement, err = lockProjectRequirementTx(
					tx, instance.WorkspaceID, instance.SubjectID, taskPlanProject.ID,
				)
				if err != nil || !requirementMatchesProject(taskPlanRequirement, taskPlanProject) {
					return fmt.Errorf("%w: requirement scope changed", errTaskPlanApplyStale)
				}
				aiRun, err = lockTaskPlanAIRunRowTx(tx, instance.WorkspaceID, aiRun.ID)
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errAITransition
				}
				if err != nil {
					return err
				}
			}
			if !reusedAIResult && node.Capability == "requirement.completeness" && (aiRun.Status == "completed" || aiRun.Status == "applied") {
				var requirement requirementgorm.Requirement
				if err := tx.Select("version").Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).First(&requirement).Error; err != nil {
					return err
				}
				if aiRun.SubjectVersion != requirement.Version {
					return errWorkflowAIStale
				}
			}
			switch aiRun.Status {
			case "queued", "running":
				return errWorkflowAIWaiting
			case "completed", "applied":
				if node.Capability == taskplan.Capability && aiRun.Status == "completed" {
					if skipTaskPlan {
						// The requirement keeps its already-created tasks and the
						// node completes without applying the generated plan.
						var existing int64
						if err := tx.Model(&taskgorm.WorkTask{}).
							Where("workspace_id = ? AND requirement_id = ? AND LOWER(TRIM(kind)) IN ?",
								instance.WorkspaceID, instance.SubjectID, taskgorm.PMWorkTaskKinds()).
							Count(&existing).Error; err != nil {
							return err
						}
						if existing == 0 {
							return errTaskPlanSkipUnavailable
						}
					} else {
						// Workflow-bound plans keep the lane-owner contract: the
						// requirement's frontend/backend owners become assignees.
						laneAssignee := func(lane string) *uint {
							return requirementgorm.TaskPlanLaneAssignee(taskPlanRequirement, lane)
						}
						if _, err := s.applyTaskPlanRunTx(tx, c, &aiRun, taskPlanProject, taskPlanRequirement, now, laneAssignee); err != nil {
							return err
						}
					}
				}
				status = "completed"
				resultPath := workflowAIResultPath(node.Capability, aiRun.OutputJSON)
				if aiPathOverride != "" {
					resultPath = aiPathOverride
				}
				next = workflowNextNodes(definition, node.ID, resultPath)
				if aiPathOverride != "" {
					run.OutputJSON = encodeWorkflowRunOverride(currentFactUserID(c), comment, workflowSingleTransitionTarget(next))
				} else {
					run.OutputJSON = encodeWorkflowRunTransition(0, "", workflowSingleTransitionTarget(next))
				}
			case "failed", "cancelled":
				if aiPathOverride != "" {
					return errWorkflowAIOverrideUnavailable
				}
				return errWorkflowAIRetryRequired
			default:
				return errors.New("workflow AI run has an unsupported status")
			}
			run.Status, run.CompletedAt = status, &now
			if err := tx.Model(&workflowgorm.WorkflowNodeRun{}).Where("id = ? AND workspace_id = ?", run.ID, instance.WorkspaceID).Updates(map[string]any{"status": status, "completed_at": now, "error_message": run.ErrorMessage, "output_json": run.OutputJSON}).Error; err != nil {
				return err
			}
			if err := tx.Model(&job).Updates(map[string]any{"status": status, "last_error": run.ErrorMessage}).Error; err != nil {
				return err
			}
		} else if err := tx.Create(&run).Error; err != nil {
			return err
		}
		if node.Type == "approval" {
			userID := currentFactUserID(c)
			approval := workflowgorm.WorkflowApproval{WorkspaceID: instance.WorkspaceID, InstanceID: instance.ID, NodeRunID: run.ID, Status: status, ApproverUserID: &userID, Comment: strings.TrimSpace(comment), DecidedAt: &now, CreatedAt: now}
			if err := tx.Create(&approval).Error; err != nil {
				return err
			}
		}
		current := removeString(currentWorkflowNodes(instance.CurrentNodeIDs), node.ID)
		current = uniqueStrings(append(current, next...))
		var err error
		current, err = s.advanceAutomaticWorkflowNodesTx(tx, &instance, definition, current, now)
		if err != nil {
			return err
		}
		if len(current) == 0 && instance.Status != "completed" {
			return errWorkflowNoTransition
		}
		if status != "rejected" {
			// 锁版语义：完成需求录入只结束当前节点，缺失的下一节点负责人默认
			// 就是操作者本人，不因未选人阻塞复合提交（用户确认的口径）。
			defaultAssignee := uint(0)
			if workflowNodeRequiresRequirementIntakeLock(instance, node) {
				defaultAssignee = currentFactUserID(c)
			}
			if err := s.assignWorkflowNextNodesTx(tx, c, &instance, definition, current, nextAssignees, defaultAssignee); err != nil {
				return err
			}
		}
		encoded, _ := json.Marshal(current)
		updates := map[string]any{"current_node_ids": string(encoded), "status": instance.Status, "completed_at": instance.CompletedAt}
		if len(current) == 0 {
			updates["status"] = "completed"
		}
		if instance.NodePlansJSON != expectedPlans {
			updates["node_plans_json"] = instance.NodePlansJSON
		}
		if err := tx.Model(&workflowgorm.WorkflowToken{}).Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", instance.WorkspaceID, instance.ID, node.ID, "processing").Update("status", status).Error; err != nil {
			return err
		}
		for _, nextID := range current {
			var count int64
			tx.Model(&workflowgorm.WorkflowToken{}).Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?", instance.WorkspaceID, instance.ID, nextID, []string{"active", "waiting"}).Count(&count)
			if count == 0 {
				if err := createWorkflowTokenTx(tx, instance, definition, nextID, "active", false); err != nil {
					if errors.Is(err, errWorkflowNodeAssigneeUnavailable) {
						return errWorkflowNextAssigneeRequired
					}
					return err
				}
			}
		}
		if err := s.applyWorkflowAssigneeTokensTx(tx, c, instance, nextAssignees); err != nil {
			return err
		}
		updated := tx.Model(&workflowgorm.WorkflowInstance{}).
			Where(
				"id = ? AND workspace_id = ? AND status = ? AND current_node_ids = ? AND node_plans_json = ?",
				instance.ID, instance.WorkspaceID, expectedStatus, expectedCurrent, expectedPlans,
			).
			Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errWorkflowActionChanged
		}
		if err := syncWorkflowSubjectStatus(tx, instance, definition, current); err != nil {
			return err
		}
		if err := s.queueEnteredWorkflowAINodesTx(tx, c, instance, definition, current, now); err != nil {
			return err
		}
		projectID := s.workflowSubjectProjectID(tx, instance.WorkspaceID, instance.SubjectType, instance.SubjectID)
		// The intake lock lands after the instance CAS: plan-unit last keeps a
		// single lock order (project set -> workflow rows -> plan unit) with
		// schedule and requirement paths.
		if err := s.lockRequirementIntakeTx(tx, c, instance, node, status, confirmRequirementLock, projectID, now); err != nil {
			return err
		}
		if designURL != "" && instance.SubjectType == "requirement" {
			var requirement requirementgorm.Requirement
			if err := tx.Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).First(&requirement).Error; err != nil {
				return errors.New("workflow requirement is unavailable")
			}
			if requirement.DesignURL != designURL {
				if err := tx.Model(&requirement).Updates(map[string]any{"design_url": designURL, "version": gorm.Expr("version + 1")}).Error; err != nil {
					return err
				}
				if err := s.recordMutation(tx, c, projectID, "requirement", instance.SubjectID, "design_attached", node.Name, gin.H{"design_url": designURL, "workflow_instance_id": instance.ID, "node_id": node.ID}); err != nil {
					return err
				}
			}
		}
		// Requirement evidence changed: ask the delivery consumer to
		// re-evaluate the scheduled plan unit's gates and progress.
		if instance.SubjectType == "requirement" {
			if unitID := requirementScheduledPlanUnitTx(tx, instance.WorkspaceID, instance.SubjectID); unitID > 0 {
				if err := recordDeliveryReconcileTx(tx, instance.WorkspaceID, unitID, "requirement_node_action:"+status); err != nil {
					return err
				}
			}
		}
		return s.recordMutation(tx, c, projectID, "workflow_instance", instance.ID, status, node.Name, gin.H{"node_id": node.ID, "node_type": node.Type, "action": status, "manual_override": aiPathOverride != "", "comment": strings.TrimSpace(comment)})
	}); err != nil {
		var guardBlocked *workflowRequirementsGuardBlocked
		if errors.As(err, &guardBlocked) {
			c.JSON(http.StatusConflict, gin.H{
				"code":               apiErrorWorkflowRequirementsGuardBlocked,
				"error":              "需求达标条件未满足，暂时无法完成该节点；已满足 " + strconv.Itoa(guardBlocked.evaluation.SatisfiedCount) + "/" + strconv.Itoa(guardBlocked.evaluation.TotalCount),
				"requirements_guard": guardBlocked.evaluation,
			})
			return
		}
		if errors.Is(err, errTaskPlanApplyInvalid) || errors.Is(err, errTaskPlanApplyStale) || errors.Is(err, errAITransition) {
			respondTaskPlanApplyError(c, err)
			return
		}
		if errors.Is(err, errTaskPlanSkipUnavailable) {
			fail(c, http.StatusConflict, "the requirement has no existing tasks; use the AI plan to create them first")
			return
		}
		if errors.Is(err, errWorkflowActionChanged) {
			fail(c, http.StatusConflict, err.Error())
			return
		}
		if workflowNodePlanDatabaseConflict(err) {
			fail(c, http.StatusConflict, "workflow changed; reload and retry")
			return
		}
		if errors.Is(err, errWorkflowAIWaiting) {
			fail(c, http.StatusConflict, "AI run is still processing")
			return
		}
		if errors.Is(err, errWorkflowAIOverrideUnavailable) {
			fail(c, http.StatusConflict, "AI result must be completed before accepting its risk")
			return
		}
		if errors.Is(err, errWorkflowAIStale) {
			failCode(c, http.StatusConflict, workflowAIStaleCode, "AI result is stale; recheck the current requirement before continuing")
			return
		}
		if errors.Is(err, errWorkflowAIRetryRequired) {
			fail(c, http.StatusConflict, "failed or cancelled AI run must be retried")
			return
		}
		if errors.Is(err, errWorkflowNextAssigneeRequired) {
			failCode(c, http.StatusConflict, "WORKFLOW_NEXT_ASSIGNEE_REQUIRED", errWorkflowNextAssigneeRequired.Error())
			return
		}
		if errors.Is(err, errWorkflowNoTransition) {
			fail(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, projectgorm.ErrPlanUnitRequirementsLocked) {
			failCode(c, http.StatusConflict, apiErrorProjectRequirementsLocked,
				"项目已锁版，不能新建需求；已有需求仍可移入或排期")
			return
		}
		if errors.Is(err, projectgorm.ErrPlanUnitMutationInProgress) {
			fail(c, http.StatusConflict, "plan unit is being modified; reload and retry")
			return
		}
		if errors.Is(err, errAIRuntimeDisabled) {
			fail(c, http.StatusServiceUnavailable, errAIRuntimeDisabled.Error())
			return
		}
		dbFail(c)
		return
	}
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.ID, instance.WorkspaceID).First(&instance).Error
	var runs []workflowgorm.WorkflowNodeRun
	var approvals []workflowgorm.WorkflowApproval
	_ = s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&runs).Error
	_ = s.db.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).Order("id").Find(&approvals).Error
	var template workflowgorm.WorkflowTemplate
	var version workflowgorm.WorkflowTemplateVersion
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.TemplateID, instance.WorkspaceID).First(&template).Error
	_ = s.db.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).First(&version).Error
	if queuedAI {
		ok(c, attachWorkflowTemplateMeta(s.workflowInstanceDTOWithDefinition(instance, runs, approvals, definition), template, version))
		return
	}
	ok(c, attachWorkflowTemplateMeta(s.workflowInstanceDTOWithDefinition(instance, runs, approvals, definition), template, version))
}

func workflowSingleTransitionTarget(next []string) string {
	if len(next) == 1 {
		return next[0]
	}
	return ""
}
