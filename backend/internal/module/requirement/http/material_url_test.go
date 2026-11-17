package requirementhttp

import (
	"testing"

	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

func TestCanonicalDesignURLResolvesDownloadPath(t *testing.T) {
	row := requirement.Material{
		ID: 9, RequirementID: 42, Kind: requirement.MaterialKindDesign,
		URL: "https://cdn.example.test/checkout.png", ObjectKey: "pm/workspace/1/design_assets/checkout.png",
	}
	download := "/api/v1/platform/requirements/42/materials/9/download"
	if got, ok := CanonicalDesignURL([]requirement.Material{row}, download); !ok || got != row.URL {
		t.Fatalf("download path canonical=%q ok=%v", got, ok)
	}
	if got, ok := CanonicalDesignURL([]requirement.Material{row}, "https://pm.example.test"+download); !ok || got != row.URL {
		t.Fatalf("absolute download path canonical=%q ok=%v", got, ok)
	}
	if got, ok := CanonicalDesignURL([]requirement.Material{row}, row.URL); !ok || got != row.URL {
		t.Fatalf("origin url canonical=%q ok=%v", got, ok)
	}
	if _, ok := CanonicalDesignURL([]requirement.Material{row}, "/api/v1/platform/requirements/42/materials/8/download"); ok {
		t.Fatal("foreign material download path must not resolve")
	}
}
