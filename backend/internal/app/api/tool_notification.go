package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"gorm.io/gorm"
)

func toolNotificationUsers(
	ctx context.Context,
	tx *gorm.DB,
	workspaceID uint,
	referencedUserIDs ...uint,
) (map[uint]identity.ToolUser, error) {
	service, err := identity.NewToolDirectoryService(identitygorm.NewToolDirectoryRepository(tx))
	if err != nil {
		return nil, err
	}
	page, err := service.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	result := make(map[uint]identity.ToolUser, len(page.Items))
	for _, user := range page.Items {
		result[user.ID] = user
	}
	// Display-only lookup for authoritative IDs already referenced by the mutation.
	// Membership controls recipients separately; this does not grant access.
	directory := identitygorm.NewAccessGuideDirectory(tx)
	for _, id := range referencedUserIDs {
		if id == 0 {
			continue
		}
		if _, ok := result[id]; ok {
			continue
		}
		name, err := directory.GuideOwnerName(ctx, id)
		if err != nil {
			return nil, err
		}
		if name != "" {
			result[id] = identity.ToolUser{ID: id, DisplayName: name}
		}
	}
	return result, nil
}

func toolNotificationUserLabel(users map[uint]identity.ToolUser, userID uint) string {
	if userID == 0 {
		return "-"
	}
	if user, exists := users[userID]; exists {
		if label := strings.TrimSpace(user.DisplayName); label != "" {
			return label
		}
		if label := strings.TrimSpace(user.Username); label != "" {
			return label
		}
	}
	return fmt.Sprintf("用户#%d", userID)
}

func toolNotificationRecipients(actorID uint, includeActor bool, groups ...[]uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0)
	for _, group := range groups {
		for _, userID := range group {
			if userID == 0 || !includeActor && userID == actorID {
				continue
			}
			if _, exists := seen[userID]; exists {
				continue
			}
			seen[userID] = struct{}{}
			result = append(result, userID)
		}
	}
	return result
}
