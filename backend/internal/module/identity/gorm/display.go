package identitygorm

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
	"strings"
)

type UserDisplayRepository struct{ db *gorm.DB }

func NewUserDisplayRepository(db *gorm.DB) *UserDisplayRepository {
	return &UserDisplayRepository{db: db}
}

// Only references obtained from an authorized resource may be passed here.
// Removed workspace membership does not erase historical authorship.
func (r *UserDisplayRepository) ReadUserDisplays(ctx context.Context, ids []uint) (map[uint]identity.UserDisplay, error) {
	out := map[uint]identity.UserDisplay{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []User
	if err := r.db.WithContext(ctx).Select("id, username, display_name, avatar_url").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		name := strings.TrimSpace(row.DisplayName)
		if name == "" {
			name = strings.TrimSpace(row.Username)
		}
		out[row.ID] = identity.UserDisplay{ID: row.ID, Name: name, Username: row.Username, AvatarURL: row.AvatarURL}
	}
	return out, nil
}
