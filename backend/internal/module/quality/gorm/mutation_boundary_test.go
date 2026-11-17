package qualitygorm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestQualityMutationBoundaryDoesNotLeakGORMAcrossDomainContracts(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	gormSource, err := os.ReadFile(filepath.Join(filepath.Dir(current), "mutation_repository.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gormSource), "module/requirement/gorm") {
		t.Fatal("quality GORM adapter must consume requirement domain facts, not requirement GORM")
	}
	moduleRoot := filepath.Dir(filepath.Dir(current))
	for _, path := range []string{
		filepath.Join(moduleRoot, "mutation.go"),
		filepath.Join(filepath.Dir(moduleRoot), "requirement", "mutation_scope.go"),
	} {
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(source), "gorm.io/") || strings.Contains(string(source), "*gorm.DB") {
			t.Fatalf("domain contract leaks GORM: %s", path)
		}
	}
}
