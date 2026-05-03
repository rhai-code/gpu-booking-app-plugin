package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsGPUResource(t *testing.T) {
	known := []string{
		"nvidia.com/gpu",
		"nvidia.com/mig-3g.71gb",
		"nvidia.com/mig-2g.35gb",
		"nvidia.com/mig-1g.18gb",
	}
	for _, r := range known {
		if !IsGPUResource(r) {
			t.Errorf("expected known GPU resource: %q", r)
		}
	}

	unknown := []string{
		"",
		"nvidia.com/mig-7g.999gb",
		"cpu",
		"memory",
		"nvidia.com/GPU",
	}
	for _, r := range unknown {
		if IsGPUResource(r) {
			t.Errorf("expected unknown GPU resource: %q", r)
		}
	}
}

func TestGPUSpecByType(t *testing.T) {
	spec, ok := GPUSpecByType("nvidia.com/gpu")
	if !ok {
		t.Fatal("expected ok for nvidia.com/gpu")
	}
	if spec.Name != "H200 Full GPU" {
		t.Errorf("Name = %q, want H200 Full GPU", spec.Name)
	}
	if spec.Count != 8 {
		t.Errorf("Count = %d, want 8", spec.Count)
	}
	if spec.GPUEquivalent != 1.0 {
		t.Errorf("GPUEquivalent = %f, want 1.0", spec.GPUEquivalent)
	}

	spec, ok = GPUSpecByType("nvidia.com/mig-1g.18gb")
	if !ok {
		t.Fatal("expected ok for mig-1g.18gb")
	}
	if spec.Count != 16 {
		t.Errorf("Count = %d, want 16", spec.Count)
	}
	if spec.GPUEquivalent != 0.125 {
		t.Errorf("GPUEquivalent = %f, want 0.125", spec.GPUEquivalent)
	}

	_, ok = GPUSpecByType("nonexistent")
	if ok {
		t.Error("expected !ok for nonexistent type")
	}
}

func TestGetConfig(t *testing.T) {
	cfg := GetConfig(30)
	if cfg.BookingWindowDays != 30 {
		t.Errorf("BookingWindowDays = %d, want 30", cfg.BookingWindowDays)
	}
	if len(cfg.Resources) == 0 {
		t.Fatal("expected non-empty Resources")
	}
	if cfg.TotalCPU != TotalCPU {
		t.Errorf("TotalCPU = %d, want %d", cfg.TotalCPU, TotalCPU)
	}
	if cfg.TotalMemory != TotalMemory {
		t.Errorf("TotalMemory = %d, want %d", cfg.TotalMemory, TotalMemory)
	}
}

func TestInitAndClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	if err := Init(dbPath); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer Close()

	if DB() == nil {
		t.Fatal("DB() returned nil after Init")
	}

	if err := DB().Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestInitCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "dir", "test.db")

	if err := Init(nested); err != nil {
		t.Fatalf("Init with nested path: %v", err)
	}
	defer Close()

	if _, err := os.Stat(filepath.Dir(nested)); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

func TestInsertAndQueryBooking(t *testing.T) {
	dir := t.TempDir()
	if err := Init(filepath.Join(dir, "test.db")); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer Close()

	_, err := DB().Exec(
		"INSERT INTO bookings ("+BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "alice", "alice@test.com", "nvidia.com/gpu", 0,
		"2025-04-24", "full", "2025-04-24T00:00:00Z", "reserved", "test booking", 0, 24, 0,
	)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	rows, err := DB().Query("SELECT " + BookingColumns + " FROM bookings WHERE id = ?", "booking-1")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected one row")
	}
	b, err := ScanBooking(rows)
	if err != nil {
		t.Fatalf("ScanBooking: %v", err)
	}
	if b.ID != "booking-1" {
		t.Errorf("ID = %q, want booking-1", b.ID)
	}
	if b.User != "alice" {
		t.Errorf("User = %q, want alice", b.User)
	}
	if b.Resource != "nvidia.com/gpu" {
		t.Errorf("Resource = %q, want nvidia.com/gpu", b.Resource)
	}
	if b.StartHour != 0 || b.EndHour != 24 {
		t.Errorf("Hours = %d-%d, want 0-24", b.StartHour, b.EndHour)
	}
}

func TestUniqueConstraint(t *testing.T) {
	dir := t.TempDir()
	if err := Init(filepath.Join(dir, "test.db")); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer Close()

	insert := "INSERT INTO bookings (" + BookingColumns + ") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)"
	args := []any{"booking-1", "alice", "", "nvidia.com/gpu", 0, "2025-04-24", "full", "now", "reserved", "", 0, 24, 0}

	if _, err := DB().Exec(insert, args...); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// same resource+slot+date+type should fail
	args[0] = "booking-2" // different ID
	_, err := DB().Exec(insert, args...)
	if err == nil {
		t.Error("expected unique constraint violation")
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	origSpecs := make([]GPUResourceSpec, len(GPUResourceSpecs))
	copy(origSpecs, GPUResourceSpecs)
	origCPU := TotalCPU
	origMem := TotalMemory
	defer func() {
		GPUResourceSpecs = origSpecs
		TotalCPU = origCPU
		TotalMemory = origMem
	}()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gpu.json")

	content := `{
		"resources": [
			{"name": "Test GPU", "type": "test/gpu", "count": 4, "share": 0.125, "gpuEquivalent": 2.0}
		],
		"totalCpu": 128,
		"totalMemory": 2048
	}`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := LoadConfigFromFile(cfgPath); err != nil {
		t.Fatalf("LoadConfigFromFile: %v", err)
	}

	if len(GPUResourceSpecs) != 1 || GPUResourceSpecs[0].Name != "Test GPU" {
		t.Errorf("Resources not overwritten: %+v", GPUResourceSpecs)
	}
	if TotalCPU != 128 {
		t.Errorf("TotalCPU = %d, want 128", TotalCPU)
	}
	if TotalMemory != 2048 {
		t.Errorf("TotalMemory = %d, want 2048", TotalMemory)
	}
}

func TestLoadConfigFromFileMissingFile(t *testing.T) {
	err := LoadConfigFromFile("/nonexistent/gpu.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadConfigFromFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("{invalid"), 0644)

	err := LoadConfigFromFile(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
