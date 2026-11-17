package requirement

import "testing"

func TestParseMaterialKindAndSource(t *testing.T) {
	kind, err := ParseMaterialKind(" design ")
	if err != nil || kind != MaterialKindDesign {
		t.Fatalf("kind=%q err=%v", kind, err)
	}
	if _, err := ParseMaterialKind("test_report"); err == nil {
		t.Fatal("test_report must not be a requirement material kind")
	}
	source, err := ParseMaterialSource("testing_library")
	if err != nil || source != MaterialSourceTestingLibrary {
		t.Fatalf("source=%q err=%v", source, err)
	}
	if _, err := ParseMaterialSource("workflow_node"); err == nil {
		t.Fatal("unknown source should fail")
	}
}

type memoryMaterials struct {
	rows []Material
}

func (m *memoryMaterials) Create(input MaterialInput) (Material, error) {
	row := Material{
		ID: uint(len(m.rows) + 1), WorkspaceID: input.WorkspaceID, RequirementID: input.RequirementID,
		Kind: input.Kind, Source: input.Source, Name: input.Name, ContentType: input.ContentType,
		Size: input.Size, ObjectKey: input.ObjectKey, URL: input.URL, UploaderUserID: input.UploaderUserID,
	}
	m.rows = append(m.rows, row)
	return row, nil
}
func (m *memoryMaterials) List(workspaceID, requirementID uint, kind string) ([]Material, error) {
	var out []Material
	for _, row := range m.rows {
		if row.WorkspaceID == workspaceID && row.RequirementID == requirementID && (kind == "" || row.Kind == kind) {
			out = append(out, row)
		}
	}
	return out, nil
}
func (m *memoryMaterials) Get(workspaceID, requirementID, materialID uint) (Material, error) {
	for _, row := range m.rows {
		if row.WorkspaceID == workspaceID && row.RequirementID == requirementID && row.ID == materialID {
			return row, nil
		}
	}
	return Material{}, ErrMaterialNotFound
}
func (m *memoryMaterials) Delete(workspaceID, requirementID, materialID uint) error {
	for i, row := range m.rows {
		if row.WorkspaceID == workspaceID && row.RequirementID == requirementID && row.ID == materialID {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			return nil
		}
	}
	return ErrMaterialNotFound
}
func (m *memoryMaterials) Count(workspaceID, requirementID uint, kind string) (int64, error) {
	rows, _ := m.List(workspaceID, requirementID, kind)
	return int64(len(rows)), nil
}
func (m *memoryMaterials) ListTestCases(uint, LibraryFilter) ([]LibraryMaterial, error) {
	return nil, nil
}
func (m *memoryMaterials) GetByObjectKey(workspaceID uint, objectKey string) (Material, error) {
	for _, row := range m.rows {
		if row.WorkspaceID == workspaceID && row.ObjectKey == objectKey {
			return row, nil
		}
	}
	return Material{}, ErrMaterialNotFound
}
func (m *memoryMaterials) ListForRequirements(workspaceID uint, requirementIDs []uint, kind string) ([]Material, error) {
	allowed := map[uint]struct{}{}
	for _, id := range requirementIDs {
		allowed[id] = struct{}{}
	}
	var out []Material
	for _, row := range m.rows {
		if row.WorkspaceID != workspaceID {
			continue
		}
		if _, ok := allowed[row.RequirementID]; !ok {
			continue
		}
		if kind == "" || row.Kind == kind {
			out = append(out, row)
		}
	}
	return out, nil
}

func TestMaterialServiceCreatesMultipleKinds(t *testing.T) {
	service := NewMaterialService(&memoryMaterials{})
	design, err := service.Create(MaterialInput{
		WorkspaceID: 1, RequirementID: 9, Kind: MaterialKindDesign, Source: MaterialSourceRequirement,
		Name: "a.png", ContentType: "image/png", ObjectKey: "k1", URL: "https://files.example/a.png", UploaderUserID: 3,
	})
	if err != nil || design.ID == 0 {
		t.Fatalf("design=%v err=%v", design, err)
	}
	if _, err := service.Create(MaterialInput{
		WorkspaceID: 1, RequirementID: 9, Kind: MaterialKindTestCase, Source: MaterialSourceTestingLibrary,
		Name: "cases.csv", ContentType: "text/csv", ObjectKey: "k2", URL: "https://files.example/cases.csv", UploaderUserID: 3,
	}); err != nil {
		t.Fatal(err)
	}
	hasDesign, err := service.HasKind(1, 9, MaterialKindDesign)
	if err != nil || !hasDesign {
		t.Fatalf("hasDesign=%v err=%v", hasDesign, err)
	}
	rows, err := service.List(1, 9, MaterialKindTestCase)
	if err != nil || len(rows) != 1 || rows[0].Name != "cases.csv" {
		t.Fatalf("test cases=%v err=%v", rows, err)
	}
	if _, err := service.Create(MaterialInput{WorkspaceID: 1, RequirementID: 9, Kind: "test_report", Source: MaterialSourceRequirement, Name: "x"}); err != ErrMaterialKindInvalid {
		t.Fatalf("err=%v", err)
	}
}
