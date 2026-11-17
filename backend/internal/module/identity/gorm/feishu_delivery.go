package identitygorm

import (
	"context"
	"strings"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func (r *LoginRepository) FeishuOpenIDs(ctx context.Context, userIDs []uint) (map[uint]string, error) {
	return r.lookupFeishuContacts(ctx, userIDs, func(row User) string {
		return derefString(row.FeishuOpenID)
	})
}

func (r *LoginRepository) FeishuEmails(ctx context.Context, userIDs []uint) (map[uint]string, error) {
	return r.lookupFeishuContacts(ctx, userIDs, func(row User) string {
		email := strings.TrimSpace(row.Email)
		if !strings.Contains(email, "@") {
			return ""
		}
		return email
	})
}

func (r *LoginRepository) lookupFeishuContacts(ctx context.Context, userIDs []uint, value func(User) string) (map[uint]string, error) {
	ids := uniquePositiveIDs(userIDs)
	out := make(map[uint]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []User
	if err := r.db.WithContext(ctx).
		Select("id", "feishu_open_id", "email").
		Where("active = ? AND id IN ?", true, ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if contact := strings.TrimSpace(value(row)); contact != "" {
			out[row.ID] = contact
		}
	}
	return out, nil
}

func (r *LoginRepository) SaveFeishuOpenID(ctx context.Context, userID uint, openID string) error {
	openID = strings.TrimSpace(openID)
	if userID == 0 || openID == "" {
		return nil
	}
	result := r.db.WithContext(ctx).Model(&User{}).
		Where("active = ? AND id = ?", true, userID).
		Updates(map[string]any{"feishu_open_id": openID})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func persistFeishuDeliveryIdentity(tx *gorm.DB, user *User, provider identity.FeishuIdentity) error {
	if user == nil || user.ID == 0 {
		return gorm.ErrRecordNotFound
	}
	updates := map[string]any{}
	openID := strings.TrimSpace(provider.OpenID)
	if openID != "" && derefString(user.FeishuOpenID) != openID {
		updates["feishu_open_id"] = openID
	}
	avatar := strings.TrimSpace(provider.AvatarURL)
	if avatar != "" && user.AvatarURL != avatar {
		updates["avatar_url"] = avatar
	}
	if len(updates) == 0 {
		return nil
	}
	if err := tx.Model(&User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		return err
	}
	if openID != "" {
		user.FeishuOpenID = optionalString(openID)
	}
	if avatar != "" {
		user.AvatarURL = avatar
	}
	return nil
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func uniquePositiveIDs(values []uint) []uint {
	seen := map[uint]struct{}{}
	ids := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}
	return ids
}
