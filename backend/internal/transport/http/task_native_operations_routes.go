package httpapi

import "github.com/gin-gonic/gin"

func (s *Server) mountTaskNativeOperationRoutes(scoped *gin.RouterGroup) {
	scoped.POST("/tasks/:id/clone", s.cloneNativeTask)
	scoped.POST("/tasks/batch-update/impact", s.nativeTaskBatchUpdateImpact)
	scoped.POST("/tasks/batch-update", s.batchUpdateNativeTasks)
	scoped.POST("/tasks/batch-status-update", s.batchUpdateNativeTaskStatuses)
	scoped.GET("/tasks/stats/delivery", s.nativeTaskDeliveryStats)
	scoped.GET("/tasks/stats/due-completion", s.nativeTaskDueCompletionStats)
	scoped.GET("/tasks/stats/by-assignee-status", s.nativeTaskAssigneeStatusStats)
}
