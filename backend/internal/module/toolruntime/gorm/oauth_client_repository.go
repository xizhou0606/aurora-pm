package toolruntimegorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type OAuthClientRepository struct {
	db *gorm.DB
}

func NewOAuthClientRepository(db *gorm.DB) *OAuthClientRepository {
	return &OAuthClientRepository{db: db}
}

func (repository *OAuthClientRepository) Create(ctx context.Context, client *toolruntime.OAuthClient) error {
	if repository == nil || repository.db == nil || client == nil || client.Validate() != nil {
		return toolruntime.ErrOAuthClientInvalid
	}
	redirectURIs, err := json.Marshal(client.RedirectURIs)
	if err != nil {
		return toolruntime.ErrOAuthClientInvalid
	}
	row := ExternalOAuthClient{
		ID: client.ID, Name: client.Name, Channel: string(client.Channel),
		RedirectURIs: string(redirectURIs), CreatedAt: client.CreatedAt.UTC(),
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return toolruntime.ErrOAuthClientInvalid
	}
	return nil
}

func (repository *OAuthClientRepository) FindByID(ctx context.Context, id string) (toolruntime.OAuthClient, error) {
	if repository == nil || repository.db == nil || strings.TrimSpace(id) == "" {
		return toolruntime.OAuthClient{}, toolruntime.ErrOAuthClientInvalid
	}
	var row ExternalOAuthClient
	if err := repository.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(id)).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return toolruntime.OAuthClient{}, toolruntime.ErrOAuthClientInvalid
		}
		return toolruntime.OAuthClient{}, err
	}
	var redirectURIs []string
	if json.Unmarshal([]byte(row.RedirectURIs), &redirectURIs) != nil {
		return toolruntime.OAuthClient{}, toolruntime.ErrOAuthClientInvalid
	}
	client := toolruntime.OAuthClient{
		ID: row.ID, Name: row.Name, Channel: toolruntime.DeliveryChannel(row.Channel),
		RedirectURIs: redirectURIs, CreatedAt: row.CreatedAt.UTC(),
	}
	if client.Validate() != nil {
		return toolruntime.OAuthClient{}, toolruntime.ErrOAuthClientInvalid
	}
	return client, nil
}

var _ toolruntime.OAuthClientRepository = (*OAuthClientRepository)(nil)
