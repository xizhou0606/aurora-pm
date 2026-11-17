package httpapi

import "testing"

func TestDomainStatusValidatorsStaySeparated(t *testing.T) {
	tests := []struct {
		name      string
		validate  func(string) bool
		allowed   []string
		forbidden []string
	}{
		{
			name: "project", validate: validProjectStatus,
			allowed: []string{"active", "archived"},
			forbidden: []string{
				"planning", "backlog", "development", "released", "done",
			},
		},
		{
			name: "plan unit", validate: validPlanUnitStatus,
			allowed: []string{
				"planning", "grooming", "development", "acceptance", "released", "archived",
			},
			forbidden: []string{"active", "backlog", "done"},
		},
		{
			name: "requirement", validate: validRequirementStatus,
			allowed:   []string{"backlog", "grooming", "development", "acceptance", "done"},
			forbidden: []string{"active", "planning", "released", "archived"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, status := range test.allowed {
				if !test.validate(status) {
					t.Fatalf("status %q should be allowed", status)
				}
			}
			for _, status := range test.forbidden {
				if test.validate(status) {
					t.Fatalf("status %q should be forbidden", status)
				}
			}
		})
	}
}
