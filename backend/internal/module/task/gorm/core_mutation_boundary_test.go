package taskgorm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCoreMutationBoundaryDoesNotLeakGORMOrImportAnotherModuleAdapter(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	directory := filepath.Dir(current)
	for _, name := range []string{"core_mutation_repository.go", "core_mutation_apply.go", "core_mutation_helpers.go", "task_deletion_repository.go"} {
		raw, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"internal/module/project/gorm", "internal/module/requirement/gorm", "internal/module/quality/gorm", "internal/module/identity/gorm", "internal/module/audit/gorm"} {
			if strings.Contains(string(raw), forbidden) {
				t.Fatalf("%s imports forbidden adapter %q", name, forbidden)
			}
		}
	}
	for _, name := range []string{"core_mutation.go", "core_mutation_ports.go", "deletion.go"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(directory), name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "gorm.io/") || strings.Contains(string(raw), "*gorm.DB") {
			t.Fatalf("task domain file %s leaks GORM", name)
		}
	}
}
