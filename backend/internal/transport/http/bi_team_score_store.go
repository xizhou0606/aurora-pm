package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type biTeamScoreProfileRequest struct {
	ExpectedVersion *int                            `json:"expected_version"`
	ResetToDefault  bool                            `json:"reset_to_default"`
	Items           []biTeamScoreProfileItemRequest `json:"items"`
}

type biTeamScoreProfileItemRequest struct {
	MetricKey      string `json:"metric_key"`
	RoleKey        string `json:"role_key"`
	Enabled        bool   `json:"enabled"`
	WeightBPS      int    `json:"weight_bps"`
	MissingPolicy  string `json:"missing_policy"`
	MinimumSamples int    `json:"minimum_samples"`
}

func (s *Server) loadBITeamScoreProfile(workspaceID uint) (biTeamScoreProfileDTO, error) {
	var profile workbenchgorm.BITeamScoreProfile
	err := s.db.Where("workspace_id = ? AND status = ?", workspaceID, "active").
		Order("version DESC").First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultBITeamScoreProfile(), nil
	}
	if err != nil {
		return biTeamScoreProfileDTO{}, err
	}
	var items []workbenchgorm.BITeamScoreProfileItem
	if err := s.db.Where("profile_id = ?", profile.ID).Order("sort_order ASC, id ASC").
		Find(&items).Error; err != nil {
		return biTeamScoreProfileDTO{}, err
	}
	snapshot := workbench.BITeamScoreProfileSnapshot{ID: profile.ID, WorkspaceID: profile.WorkspaceID,
		Version: profile.Version, Status: profile.Status, ActivatedAt: profile.ActivatedAt,
		Items: make([]workbench.BITeamScoreProfileItemSnapshot, 0, len(items))}
	for _, item := range items {
		snapshot.Items = append(snapshot.Items, workbench.BITeamScoreProfileItemSnapshot{
			MetricKey: item.MetricKey, RoleKey: item.RoleKey, Enabled: item.Enabled, WeightBPS: item.WeightBPS,
			MissingPolicy: item.MissingPolicy, MinimumSamples: item.MinimumSamples,
			ParametersJSON: item.ParametersJSON, SortOrder: item.SortOrder,
		})
	}
	return mergeBITeamScoreProfile(snapshot), nil
}

func validateBITeamScoreProfileRequest(body biTeamScoreProfileRequest) ([]workbenchgorm.BITeamScoreProfileItem, error) {
	if body.ResetToDefault {
		defaults := defaultBITeamScoreProfile()
		body.Items = make([]biTeamScoreProfileItemRequest, 0, len(defaults.Items))
		for _, item := range defaults.Items {
			body.Items = append(body.Items, biTeamScoreProfileItemRequest{
				MetricKey: item.MetricKey, RoleKey: item.RoleKey,
				Enabled: item.Enabled, WeightBPS: item.WeightBPS,
				MissingPolicy: item.MissingPolicy, MinimumSamples: item.MinimumSamples,
			})
		}
	}
	if len(body.Items) == 0 {
		return nil, fmt.Errorf("items required")
	}
	legacy := true
	for _, input := range body.Items {
		if strings.TrimSpace(input.RoleKey) != "" && strings.TrimSpace(input.RoleKey) != biTeamScoreRoleAll {
			legacy = false
			break
		}
	}
	if legacy {
		byKey := make(map[string]biTeamScoreProfileItemRequest, len(body.Items))
		for _, input := range body.Items {
			byKey[strings.TrimSpace(input.MetricKey)] = input
		}
		expanded := make([]biTeamScoreProfileItemRequest, 0, len(biTeamMetricCatalog))
		for _, definition := range biTeamMetricCatalog {
			input, exists := byKey[definition.Key]
			if !exists {
				input = biTeamScoreProfileItemRequest{
					MetricKey: definition.Key, MissingPolicy: biTeamScoreMissingRenormalize,
					MinimumSamples: definition.DefaultMinSamples,
				}
			}
			input.RoleKey = definition.RoleKey
			expanded = append(expanded, input)
		}
		body.Items = expanded
	}
	seen := map[string]bool{}
	items := make([]workbenchgorm.BITeamScoreProfileItem, 0, len(body.Items))
	totals := map[string]int{}
	coreTotals := map[string]int{}
	for _, input := range body.Items {
		key := strings.TrimSpace(input.MetricKey)
		roleKey := strings.ToLower(strings.TrimSpace(input.RoleKey))
		definition, known := biTeamMetricDefinitionByKeyRole(key, roleKey)
		identity := roleKey + ":" + key
		if !known || seen[identity] {
			return nil, fmt.Errorf("items contain an unknown or duplicate role_key and metric_key")
		}
		seen[identity] = true
		if input.WeightBPS < 0 || input.WeightBPS > biTeamScoreTotalWeightBPS {
			return nil, fmt.Errorf("weight_bps must be between 0 and 10000")
		}
		if input.Enabled && input.WeightBPS == 0 {
			return nil, fmt.Errorf("enabled metrics require a positive weight_bps")
		}
		policy := strings.TrimSpace(input.MissingPolicy)
		if policy == "" {
			policy = biTeamScoreMissingRenormalize
		}
		if policy != biTeamScoreMissingRenormalize && policy != biTeamScoreMissingIneligible {
			return nil, fmt.Errorf("missing_policy must be renormalize or ineligible")
		}
		minimumSamples := input.MinimumSamples
		if minimumSamples == 0 {
			minimumSamples = definition.DefaultMinSamples
		}
		if minimumSamples < 1 || minimumSamples > 10_000 {
			return nil, fmt.Errorf("minimum_samples must be between 1 and 10000")
		}
		if input.Enabled {
			totals[roleKey] += input.WeightBPS
			if key == "workload_share" || key == "completion_rate" {
				coreTotals[roleKey] += input.WeightBPS
			}
		}
		items = append(items, workbenchgorm.BITeamScoreProfileItem{
			MetricKey: key, RoleKey: roleKey, Enabled: input.Enabled,
			WeightBPS: input.WeightBPS, MissingPolicy: policy,
			MinimumSamples: minimumSamples, ParametersJSON: "{}", SortOrder: definition.SortOrder,
		})
	}
	if len(seen) != len(biTeamMetricCatalog) {
		return nil, fmt.Errorf("items must include every catalog metric")
	}
	for _, roleKey := range biTeamScoreRoles {
		if totals[roleKey] != biTeamScoreTotalWeightBPS {
			return nil, fmt.Errorf("enabled metric weights for %s must total 10000 basis points", roleKey)
		}
		if !legacy && coreTotals[roleKey] < biTeamScoreCoreMinimumBPS {
			return nil, fmt.Errorf("workload and completion weights for %s must total at least 5000 basis points", roleKey)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].SortOrder < items[j].SortOrder })
	return items, nil
}

func (s *Server) putBITeamScoreProfile(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body biTeamScoreProfileRequest
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if body.ExpectedVersion == nil || *body.ExpectedVersion < 0 {
		bad(c, "expected_version required")
		return
	}
	items, err := validateBITeamScoreProfileRequest(body)
	if err != nil {
		bad(c, err.Error())
		return
	}
	a, _ := currentActor(c)
	var created workbenchgorm.BITeamScoreProfile
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var current workbenchgorm.BITeamScoreProfile
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ? AND status = ?", workspace.ID, "active").
			Order("version DESC").First(&current).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		currentVersion := 0
		if findErr == nil {
			currentVersion = current.Version
		}
		if body.ExpectedVersion != nil && *body.ExpectedVersion != currentVersion {
			return errBITeamScoreVersionConflict
		}
		now := time.Now().UTC()
		created = workbenchgorm.BITeamScoreProfile{
			WorkspaceID: workspace.ID, Version: currentVersion + 1, Status: "active",
			UpdatedByUserID: factUserID(a.User), ActivatedAt: &now,
		}
		if current.ID > 0 {
			created.BasedOnProfileID = &current.ID
			if err := tx.Model(&current).Updates(map[string]any{"status": "archived"}).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		for index := range items {
			items[index].ProfileID = created.ID
		}
		if err := tx.Create(&items).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, 0, "bi_team_score_profile", created.ID,
			"updated", "团队综合指标权重", gin.H{"version": created.Version, "items": items})
	})
	if errors.Is(err, errBITeamScoreVersionConflict) {
		failCode(c, http.StatusConflict, apiErrorBITeamScoreVersionConflict, "score profile has changed; reload and retry")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	profile, err := s.loadBITeamScoreProfile(workspace.ID)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, profile)
}

var errBITeamScoreVersionConflict = errors.New("BI team score profile version conflict")
