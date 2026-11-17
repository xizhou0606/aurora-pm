package httpapi

import (
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

type nativeTaskBatchUpdateImpact struct {
	RequestedIDs               []uint
	AutoCompletedAncestorIDs   []uint
	DuePropagatedDescendantIDs []uint
	AffectedIDs                []uint
}

func (s *Server) nativeTaskBatchUpdateImpact(c *gin.Context) {
	request, valid := s.prepareNativeTaskBatchUpdateRequest(c)
	if !valid {
		return
	}
	tasks := make([]taskgorm.WorkTask, 0, len(request.Prepared))
	for _, item := range request.Prepared {
		tasks = append(tasks, item.Before)
	}
	if !s.requireTaskCreatorAction(c, accesscatalog.ActionWorkItemsTasksBatchUpdate, tasks...) {
		return
	}
	impact, err := s.calculateNativeTaskBatchUpdateImpact(
		request.Workspace.ID, request.IDs, request.Prepared,
	)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"requested_ids":                 impact.RequestedIDs,
		"auto_completed_ancestor_ids":   impact.AutoCompletedAncestorIDs,
		"due_propagated_descendant_ids": impact.DuePropagatedDescendantIDs,
		"affected_ids":                  impact.AffectedIDs,
		"affected_count":                len(impact.AffectedIDs),
		"atomic":                        true,
	})
}

func (s *Server) calculateNativeTaskBatchUpdateImpact(
	workspaceID uint,
	requestedIDs []uint,
	prepared []preparedNativeTaskUpdate,
) (nativeTaskBatchUpdateImpact, error) {
	impact := nativeTaskBatchUpdateImpact{RequestedIDs: append([]uint(nil), requestedIDs...)}
	var tasks []taskgorm.WorkTask
	query := pmTaskQuery(s.db).Where("workspace_id = ?", workspaceID)
	query = applyWorkspaceHierarchyScope(query, "work_tasks", workspaceID)
	if err := query.Find(&tasks).Error; err != nil {
		return nativeTaskBatchUpdateImpact{}, err
	}
	byID := make(map[uint]taskgorm.WorkTask, len(tasks))
	childrenByParent := make(map[uint][]uint)
	currentDue := make(map[uint]bool, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
		currentDue[task.ID] = task.DueAt != nil
		if task.ParentID != nil && *task.ParentID > 0 {
			childrenByParent[*task.ParentID] = append(childrenByParent[*task.ParentID], task.ID)
		}
	}
	requested := make(map[uint]struct{}, len(requestedIDs))
	for _, id := range requestedIDs {
		requested[id] = struct{}{}
	}
	for _, item := range prepared {
		task := byID[item.After.ID]
		task.Status = item.After.Status
		byID[item.After.ID] = task
	}
	duePropagated := map[uint]struct{}{}
	for _, item := range prepared {
		if _, dueUpdated := item.Updates["due_at"]; !dueUpdated || item.After.DueAt == nil {
			continue
		}
		currentDue[item.After.ID] = true
		for _, descendantID := range nativeTaskDescendantIDs(item.After.ID, childrenByParent) {
			if currentDue[descendantID] {
				continue
			}
			currentDue[descendantID] = true
			duePropagated[descendantID] = struct{}{}
		}
	}
	autoCompleted := map[uint]struct{}{}
	for _, item := range prepared {
		if !isTerminalTaskStatus(item.After.Status) {
			continue
		}
		parentID := item.After.ParentID
		seen := map[uint]struct{}{}
		for parentID != nil && *parentID > 0 {
			if _, exists := seen[*parentID]; exists {
				break
			}
			seen[*parentID] = struct{}{}
			parent, exists := byID[*parentID]
			if !exists {
				break
			}
			if len(childrenByParent[parent.ID]) == 0 {
				break
			}
			allChildrenTerminal := true
			for _, childID := range childrenByParent[parent.ID] {
				child, exists := byID[childID]
				if !exists || !isTerminalTaskStatus(child.Status) {
					allChildrenTerminal = false
					break
				}
			}
			if !allChildrenTerminal {
				break
			}
			if !isTerminalTaskStatus(parent.Status) {
				parent.Status = "completed"
				byID[parent.ID] = parent
				if _, isRequested := requested[parent.ID]; !isRequested {
					autoCompleted[parent.ID] = struct{}{}
				}
			}
			parentID = parent.ParentID
		}
	}
	impact.AutoCompletedAncestorIDs = sortedNativeTaskIDs(autoCompleted)
	impact.DuePropagatedDescendantIDs = sortedNativeTaskIDs(duePropagated)
	affected := make(map[uint]struct{}, len(requestedIDs)+len(autoCompleted)+len(duePropagated))
	for _, id := range requestedIDs {
		affected[id] = struct{}{}
	}
	for id := range autoCompleted {
		affected[id] = struct{}{}
	}
	for id := range duePropagated {
		affected[id] = struct{}{}
	}
	impact.AffectedIDs = sortedNativeTaskIDs(affected)
	return impact, nil
}

func nativeTaskDescendantIDs(rootID uint, childrenByParent map[uint][]uint) []uint {
	seen := map[uint]struct{}{rootID: {}}
	frontier := append([]uint(nil), childrenByParent[rootID]...)
	result := make([]uint, 0)
	for len(frontier) > 0 {
		id := frontier[0]
		frontier = frontier[1:]
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
		frontier = append(frontier, childrenByParent[id]...)
	}
	return result
}

func sortedNativeTaskIDs(values map[uint]struct{}) []uint {
	result := make([]uint, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
