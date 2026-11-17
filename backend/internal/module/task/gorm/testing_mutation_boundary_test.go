package taskgorm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTestingMutationBoundaryKeepsGORMInsideTaskInfrastructure(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	adapter, err := os.ReadFile(filepath.Join(filepath.Dir(current), "testing_mutation_repository.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(adapter)
	if strings.Contains(source, "internal/module/project/gorm") || strings.Contains(source, "internal/module/requirement/gorm") ||
		strings.Contains(source, "internal/module/quality/gorm") || strings.Contains(source, "internal/module/identity/gorm") {
		t.Fatal("task testing mutation adapter must not import another module's GORM package")
	}
	domain, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(current)), "testing_mutation.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(domain), "gorm.io/") || strings.Contains(string(domain), "*gorm.DB") {
		t.Fatal("task testing domain contract leaks GORM")
	}
}
