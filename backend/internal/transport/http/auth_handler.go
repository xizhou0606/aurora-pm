package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(c *gin.Context) {
	if s.cfg.AuthMode == "shared_jwt" {
		fail(c, http.StatusForbidden, "local login is disabled")
		return
	}
	var body loginBody
	if err := c.ShouldBindJSON(&body); err != nil {
		bad(c, "invalid body")
		return
	}
	user, err := identitygorm.NewLoginRepository(s.db).FindPasswordLoginUser(c.Request.Context(), strings.TrimSpace(body.Username))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fail(c, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)) != nil {
		fail(c, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := s.tokens.SignLocal(user.ID, user.Username, user.Role)
	if err != nil {
		fail(c, http.StatusInternalServerError, "token signing failed")
		return
	}
	ok(c, gin.H{"token": token, "user": user})
}

func (s *Server) me(c *gin.Context) {
	a, _ := currentActor(c)
	ok(c, a.User)
}

func (s *Server) logout(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
