package taskgorm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCollaborationMutationBoundaryKeepsGORMInsideTaskInfrastructure(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	adapter, err := os.ReadFile(filepath.Join(filepath.Dir(current), "collaboration_mutation_repository.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(adapter)
	for _, forbidden := range []string{
		"internal/module/project/gorm", "internal/module/requirement/gorm", "internal/module/quality/gorm", "internal/module/identity/gorm",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("task collaboration mutation adapter imports forbidden package %q", forbidden)
		}
	}
	domain, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(current)), "collaboration_mutation.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(domain), "gorm.io/") || strings.Contains(string(domain), "*gorm.DB") {
		t.Fatal("task collaboration domain contract leaks GORM")
	}
}
