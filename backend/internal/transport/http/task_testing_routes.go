package httpapi

import "github.com/gin-gonic/gin"

// mountTaskTestingRoutes is intentionally separate from server.go so the
// testing domain can be integrated with one explicit mount call.
func (s *Server) mountTaskTestingRoutes(scoped *gin.RouterGroup) {
	scoped.GET("/tasks/:id/test-definition", s.getTaskTestDefinition)
	scoped.PUT("/tasks/:id/test-definition", s.putTaskTestDefinition)
	scoped.GET("/tasks/:id/test-context", s.getTaskTestContext)
	scoped.POST("/tasks/:id/test-executions", s.createTaskTestExecution)
	scoped.GET("/tasks/:id/test-execution-runs", s.listTaskTestExecutionRuns)
	scoped.PATCH("/tasks/:id/test-execution-runs/:runId", s.patchTaskTestExecutionRun)
}
