package api

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/pmtools"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

func assembleQualityUploadTool(db *gorm.DB, cfg config.ObjectStorage) (pmtools.QualityUploadHandler, error) {
	scopes, err := requirement.NewMutationScopeService(requirementgorm.NewMutationScopeRepository(db))
	if err != nil {
		return pmtools.QualityUploadHandler{}, err
	}
	access, err := newProjectAccessService(db)
	if err != nil {
		return pmtools.QualityUploadHandler{}, err
	}
	store, err := objectstore.New(cfg)
	if err != nil {
		return pmtools.QualityUploadHandler{}, err
	}
	return pmtools.QualityUploadHandler{Service: &quality.MaterialUploadService{Users: projectgorm.NewFactUserResolver(db), Scopes: scopes, Access: access, Store: store, Prefix: cfg.GlobalPrefix, PublicURL: func(key string) (string, bool) { return objectstore.PublicURL(cfg, key) }}}, nil
}
