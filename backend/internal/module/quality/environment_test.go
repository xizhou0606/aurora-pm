package quality

import "testing"

func TestNormalizeSceneOptionName(t *testing.T) {
	name, normalized, err := NormalizeSceneOptionName("  Pre Release  ")
	if err != nil || name != "Pre Release" || normalized != "pre release" {
		t.Fatalf("normalized=(%q, %q, %v)", name, normalized, err)
	}
	for _, value := range []string{"", "test@prod", "测试\n环境"} {
		if _, _, err := NormalizeSceneOptionName(value); err == nil {
			t.Fatalf("name %q should be invalid", value)
		}
	}
}

func TestSceneOptionKindsExposeOnlySafeBugColumns(t *testing.T) {
	for raw, column := range map[string]string{
		"environment":      "found_environment",
		"operating_system": "operating_system",
		"browser":          "browser",
	} {
		kind, err := ParseSceneOptionKind(raw)
		if err != nil || kind.LegacyColumn() != column {
			t.Fatalf("kind %q=(%q, %q, %v)", raw, kind, kind.LegacyColumn(), err)
		}
	}
	if _, err := ParseSceneOptionKind("found_environment; DROP TABLE quality_bugs"); err == nil {
		t.Fatal("unsafe kind should be rejected")
	}
}
