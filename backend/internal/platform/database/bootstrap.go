package database

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func Bootstrap(db *gorm.DB, spec config.Bootstrap) error {
	if err := bootstrap(db, spec, bcrypt.GenerateFromPassword); err != nil {
		return err
	}
	return bootstrapPersonalDocumentSpaces(db)
}

func bootstrap(
	db *gorm.DB,
	spec config.Bootstrap,
	hashPassword func([]byte, int) ([]byte, error),
) error {
	username := strings.TrimSpace(spec.Username)
	if username == "" {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var user identitygorm.User
		err := tx.Where("username = ?", username).First(&user).Error
		switch {
		case err == nil:
			localSubject := "local:" + username
			legacyLocal := user.ExternalSubject == "" && user.ExternalIssuer == "" && user.PasswordHash != ""
			if user.ExternalSubject != localSubject && !legacyLocal {
				return fmt.Errorf("bootstrap username %q belongs to a non-local identity", username)
			}
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		default:
			hash, err := hashPassword([]byte(spec.Password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			user = identitygorm.User{
				ExternalSubject: "local:" + username,
				Username:        username,
				DisplayName:     strings.TrimSpace(spec.DisplayName),
				Email:           strings.TrimSpace(spec.Email),
				PasswordHash:    string(hash),
				Role:            "super_admin",
				Active:          true,
				Status:          "active",
				SessionVersion:  1,
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		}
		if err := identitygorm.EnsureUserInDefaultGroup(tx, user.ID); err != nil {
			return err
		}
		var defaultWorkspaceCount int64
		if err := tx.Model(&identitygorm.Workspace{}).Where("is_default = ?", true).Count(&defaultWorkspaceCount).Error; err != nil {
			return err
		}
		workspace := identitygorm.Workspace{
			Name:      strings.TrimSpace(spec.WorkspaceName),
			Slug:      strings.TrimSpace(spec.WorkspaceSlug),
			Enabled:   true,
			IsDefault: defaultWorkspaceCount == 0,
		}
		err = tx.Where("slug = ?", workspace.Slug).First(&workspace).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			workspace = identitygorm.Workspace{
				Name:      strings.TrimSpace(spec.WorkspaceName),
				Slug:      strings.TrimSpace(spec.WorkspaceSlug),
				Enabled:   true,
				IsDefault: defaultWorkspaceCount == 0,
			}
			if err := tx.Create(&workspace).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if defaultWorkspaceCount == 0 && !workspace.IsDefault {
			if err := tx.Model(&workspace).Updates(map[string]any{"enabled": true, "is_default": true}).Error; err != nil {
				return err
			}
			workspace.Enabled = true
			workspace.IsDefault = true
		}
		var teamCount int64
		if err := tx.Model(&identitygorm.Team{}).Where("workspace_id = ?", workspace.ID).Count(&teamCount).Error; err != nil {
			return err
		}
		if teamCount == 0 {
			team := identitygorm.Team{
				WorkspaceID: workspace.ID, Name: strings.TrimSpace(spec.TeamName),
				OwnerUserID: &user.ID,
			}
			if team.Name == "" {
				team.Name = "Default Team"
			}
			if err := tx.Create(&team).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
