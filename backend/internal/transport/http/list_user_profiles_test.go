package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
)

func TestRequirementListUserProfilesResolveInternalIDsWithMinimalFields(t *testing.T) {
	app := newTestApp(t, nil)
	product := app.createUser(t, 1, "product", "user", "")
	developer := app.createUser(t, 2, "developer", "user", "")
	if err := app.db.Model(&identitygorm.User{}).Where("id = ?", product.ID).Updates(map[string]any{
		"external_user_id": 101,
		"display_name":     "产品同学",
		"avatar_url":       "https://cdn.example.test/product.png",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&identitygorm.User{}).Where("id = ?", developer.ID).Updates(map[string]any{
		"external_user_id": 202,
		"display_name":     "研发同学",
		"avatar_url":       "https://cdn.example.test/developer.png",
	}).Error; err != nil {
		t.Fatal(err)
	}

	productID := product.ID
	requirements := []requirementgorm.Requirement{{
		ProductOwnerUserID: &productID,
		DeveloperUserIDs:   requirementgorm.UintList{developer.ID, developer.ID},
	}}
	profiles, err := (&Server{db: app.db}).listUserProfiles(requirementListUserIDs(requirements))
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || profiles[0].ID != product.ID || profiles[0].DisplayName != "产品同学" ||
		profiles[0].AvatarURL != "https://cdn.example.test/product.png" ||
		profiles[1].ID != developer.ID || profiles[1].DisplayName != "研发同学" {
		t.Fatalf("requirement list profiles=%+v", profiles)
	}
	raw, err := json.Marshal(profiles)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"email", "role", "external_user_id", "active"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("requirement list profile leaked %q: %s", forbidden, raw)
		}
	}
}
