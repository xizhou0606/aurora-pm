package httpapi

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var localUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{2,79}$`)

type localUserBody struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	DisplayName  string `json:"display_name"`
	Email        string `json:"email"`
	PlatformRole string `json:"platform_role"`
}

func (s *Server) createLocalUser(c *gin.Context) {
	if s.cfg.AuthMode == "shared_jwt" {
		fail(c, http.StatusForbidden, "local users are disabled")
		return
	}
	var body localUserBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if !localUsernamePattern.MatchString(body.Username) || len(body.Password) < 12 {
		bad(c, "valid username and password with at least 12 characters required")
		return
	}
	if body.PlatformRole == "" {
		body.PlatformRole = "user"
	}
	if body.PlatformRole != "user" && body.PlatformRole != "readonly" {
		bad(c, "platform_role must be user or readonly")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		dbFail(c)
		return
	}
	user := identitygorm.User{
		ExternalSubject: "local:" + strings.ToLower(body.Username),
		Username:        body.Username, DisplayName: strings.TrimSpace(body.DisplayName),
		Email: strings.TrimSpace(body.Email), PasswordHash: string(hash),
		Role: body.PlatformRole, Active: true,
		Status: string(identity.AccountPendingApproval), SessionVersion: 1,
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if err := identitygorm.EnsureUserInDefaultGroup(tx, user.ID); err != nil {
			return err
		}
		if err := identitygorm.EnsureUserInDefaultWorkspace(tx, user.ID); err != nil {
			return err
		}
		return s.recordMutation(tx, c, 0, "user", user.ID, "created", user.Username,
			gin.H{"user_id": user.ID})
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) ||
			strings.Contains(strings.ToLower(err.Error()), "unique") {
			bad(c, "username already exists")
			return
		}
		dbFail(c)
		return
	}
	created(c, user)
}

func (s *Server) updateLocalPassword(c *gin.Context) {
	if s.cfg.AuthMode == "shared_jwt" {
		fail(c, http.StatusForbidden, "local passwords are disabled")
		return
	}
	userID, okID := paramID(c, "id")
	if !okID {
		return
	}
	var user identitygorm.User
	if s.db.Where("id = ? AND active = ?", userID, true).First(&user).Error != nil {
		fail(c, http.StatusNotFound, "user not found")
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if c.ShouldBindJSON(&body) != nil || len(body.NewPassword) < 12 {
		bad(c, "new_password must contain at least 12 characters")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		dbFail(c)
		return
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&user).Update("password_hash", string(hash)).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, 0, "user", user.ID, "password_updated",
			user.Username, gin.H{"user_id": user.ID})
	})
	if err != nil {
		dbFail(c)
		return
	}
	c.Status(http.StatusNoContent)
}
