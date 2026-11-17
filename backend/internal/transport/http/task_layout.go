package httpapi

import "github.com/gin-gonic/gin"

// The migrated Task Management route uses the source application's built-in
// layout registry. No production override rows are part of the PM-owned data
// model, so an empty override map selects those exact source defaults.
func (s *Server) getTaskLayoutSettings(c *gin.Context) {
	ok(c, gin.H{"version": 1, "overrides": gin.H{}})
}
