package resourceaccessgorm

import (
	"context"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultLinkExchangeLimit  = uint(20)
	defaultLinkExchangeWindow = time.Minute
)

// LinkExchangeLimiter uses one durable fixed-window counter per workspace and
// authenticated PM user. The database upsert is atomic across application
// instances. The public port currently has no trusted source/IP dimension.
type LinkExchangeLimiter struct {
	db     *gorm.DB
	clock  resourceaccess.Clock
	limit  uint
	window time.Duration
}

func NewLinkExchangeLimiter(db *gorm.DB, clock resourceaccess.Clock) (*LinkExchangeLimiter, error) {
	if db == nil || clock == nil {
		return nil, resourceaccess.ErrUnavailable
	}
	return &LinkExchangeLimiter{db: db, clock: clock, limit: defaultLinkExchangeLimit, window: defaultLinkExchangeWindow}, nil
}

func (limiter *LinkExchangeLimiter) AllowDocumentShareLinkExchange(
	ctx context.Context, principal resourceaccess.Principal,
) (bool, error) {
	if limiter == nil || limiter.db == nil || principal.WorkspaceID == 0 || principal.UserID == 0 {
		return false, resourceaccess.ErrInvalidInput
	}
	now := limiter.clock.Now().UTC()
	windowStartedAt := now.Truncate(limiter.window)
	row := LinkExchangeLimitRecord{
		WorkspaceID: principal.WorkspaceID, UserID: principal.UserID, WindowStartedAt: windowStartedAt,
		Attempts: 1, UpdatedAt: now,
	}
	err := limiter.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workspace_id"}, {Name: "user_id"}, {Name: "window_started_at"}},
		DoUpdates: clause.Assignments(map[string]any{
			"attempts":   gorm.Expr("document_share_link_exchange_limits.attempts + 1"),
			"updated_at": now,
		}),
	}).Create(&row).Error
	if err != nil {
		return false, err
	}
	var attempts uint
	if err := limiter.db.WithContext(ctx).Model(&LinkExchangeLimitRecord{}).
		Select("attempts").Where(
		"workspace_id = ? AND user_id = ? AND window_started_at = ?",
		principal.WorkspaceID, principal.UserID, windowStartedAt,
	).Scan(&attempts).Error; err != nil {
		return false, err
	}
	return attempts <= limiter.limit, nil
}

var _ resourceaccess.LinkExchangeLimiter = (*LinkExchangeLimiter)(nil)
