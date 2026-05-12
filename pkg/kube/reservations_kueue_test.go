package kube

import (
	"testing"
	"time"

	"github.com/eformat/gpu-booking-plugin/pkg/database"
)

func setupGPUConfig(t *testing.T) {
	t.Helper()
	database.SetGPUConfig(&database.GPUConfig{
		Resources: []database.GPUResourceSpec{
			{Name: "H200 Full GPU", Type: "nvidia.com/gpu", Count: 8, Share: 0.0625, GPUEquivalent: 1.0},
			{Name: "MIG 3g.71gb", Type: "nvidia.com/mig-3g.71gb", Count: 8, Share: 0.03125, GPUEquivalent: 0.5},
			{Name: "MIG 2g.35gb", Type: "nvidia.com/mig-2g.35gb", Count: 8, Share: 0.015625, GPUEquivalent: 0.25},
			{Name: "MIG 1g.18gb", Type: "nvidia.com/mig-1g.18gb", Count: 16, Share: 0.0078125, GPUEquivalent: 0.125},
		},
		TotalCPU:    30,
		TotalMemory: 119,
		FlavorName:  "gpu-pool",
	})
}

func TestReservation_CoveredResources_IncludesAllGPUTypes(t *testing.T) {
	setupTestDB(t)
	setupGPUConfig(t)

	insertBooking(t, "b1", "user2", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 0)

	res, err := getActiveReservationsAt(mustParseTime(t, "2026-05-04T12:00:00Z"))
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 reservation, got %d", len(res))
	}

	cfg := database.GetGPUConfig()
	allGPUTypes := map[string]bool{}
	for _, spec := range cfg.Resources {
		allGPUTypes[spec.Type] = false
	}

	for gpuType := range res[0].Resources {
		if _, ok := allGPUTypes[gpuType]; ok {
			allGPUTypes[gpuType] = true
		}
	}

	for gpuType, found := range allGPUTypes {
		if !found {
			t.Errorf("coveredResources missing GPU type %s — user only reserved nvidia.com/gpu but all types must be included for Kueue preemption to work", gpuType)
		}
	}
}

func TestReservation_CoveredResources_UnreservedTypesHaveZeroCount(t *testing.T) {
	setupTestDB(t)
	setupGPUConfig(t)

	insertBooking(t, "b1", "user2", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 0)

	res, err := getActiveReservationsAt(mustParseTime(t, "2026-05-04T12:00:00Z"))
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}

	if res[0].Resources["nvidia.com/gpu"] != 1 {
		t.Errorf("nvidia.com/gpu = %d, want 1", res[0].Resources["nvidia.com/gpu"])
	}
	if res[0].Resources["nvidia.com/mig-3g.71gb"] != 0 {
		t.Errorf("nvidia.com/mig-3g.71gb = %d, want 0", res[0].Resources["nvidia.com/mig-3g.71gb"])
	}
	if res[0].Resources["nvidia.com/mig-2g.35gb"] != 0 {
		t.Errorf("nvidia.com/mig-2g.35gb = %d, want 0", res[0].Resources["nvidia.com/mig-2g.35gb"])
	}
	if res[0].Resources["nvidia.com/mig-1g.18gb"] != 0 {
		t.Errorf("nvidia.com/mig-1g.18gb = %d, want 0", res[0].Resources["nvidia.com/mig-1g.18gb"])
	}
}

func TestReservation_CPUHeadroom_MinimumTwoCPU(t *testing.T) {
	setupTestDB(t)
	setupGPUConfig(t)

	// 1 GPU with share 0.0625 on 30 CPU cluster = ceil(1.875) = 2
	// Minimum is 2, so should be 2
	insertBooking(t, "b1", "user2", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 0)

	res, err := getActiveReservationsAt(mustParseTime(t, "2026-05-04T12:00:00Z"))
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if res[0].CPU < 2 {
		t.Errorf("CPU = %d, want >= 2 (minimum headroom for sidecar containers like kube-rbac-proxy)", res[0].CPU)
	}
}

func TestReservation_MemoryHeadroom_MinimumEightGi(t *testing.T) {
	setupTestDB(t)
	setupGPUConfig(t)

	insertBooking(t, "b1", "user2", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 0)

	res, err := getActiveReservationsAt(mustParseTime(t, "2026-05-04T12:00:00Z"))
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if res[0].Memory < 8 {
		t.Errorf("Memory = %dGi, want >= 8Gi (minimum headroom for sidecar containers)", res[0].Memory)
	}
}

func TestReservation_CPUHeadroom_SmallCluster(t *testing.T) {
	setupTestDB(t)

	// Very small cluster where share * totalCPU < 1
	database.SetGPUConfig(&database.GPUConfig{
		Resources: []database.GPUResourceSpec{
			{Name: "GPU", Type: "nvidia.com/gpu", Count: 8, Share: 0.0625, GPUEquivalent: 1.0},
		},
		TotalCPU:    10,
		TotalMemory: 32,
		FlavorName:  "gpu-pool",
	})

	insertBooking(t, "b1", "user2", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 0)

	res, err := getActiveReservationsAt(mustParseTime(t, "2026-05-04T12:00:00Z"))
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}

	// share * totalCPU = 0.0625 * 10 = 0.625, ceil = 1, but minimum is 2
	if res[0].CPU < 2 {
		t.Errorf("CPU = %d on small cluster, want >= 2 (minimum headroom)", res[0].CPU)
	}
	// share * totalMemory = 0.0625 * 32 = 2, ceil = 2, but minimum is 8
	if res[0].Memory < 8 {
		t.Errorf("Memory = %dGi on small cluster, want >= 8Gi (minimum headroom)", res[0].Memory)
	}
}

func TestReservation_CohortRemaining_SubtractsReservation(t *testing.T) {
	setupTestDB(t)
	setupGPUConfig(t)

	insertBooking(t, "b1", "user2", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 0)

	res, err := getActiveReservationsAt(mustParseTime(t, "2026-05-04T12:00:00Z"))
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}

	cfg := database.GetGPUConfig()
	remainingGPUs := map[string]int{}
	for _, spec := range cfg.Resources {
		remainingGPUs[spec.Type] = spec.Count
	}
	for gpuRes, count := range res[0].Resources {
		remainingGPUs[gpuRes] -= count
	}

	// Cohort should have totalGPU - reservedGPU
	if remainingGPUs["nvidia.com/gpu"] != 7 {
		t.Errorf("remaining nvidia.com/gpu = %d, want 7 (8 total - 1 reserved)", remainingGPUs["nvidia.com/gpu"])
	}
	// MIG types unreserved, should remain at full count
	if remainingGPUs["nvidia.com/mig-3g.71gb"] != 8 {
		t.Errorf("remaining mig-3g.71gb = %d, want 8", remainingGPUs["nvidia.com/mig-3g.71gb"])
	}
}

func TestReservation_MultipleGPUTypes_CoveredResourcesComplete(t *testing.T) {
	setupTestDB(t)
	setupGPUConfig(t)

	// User reserves both full GPU and MIG slices
	insertBooking(t, "b1", "user2", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 0)
	insertBooking(t, "b2", "user2", "nvidia.com/mig-3g.71gb", 0, "2026-05-04", 0, 24, 0)

	res, err := getActiveReservationsAt(mustParseTime(t, "2026-05-04T12:00:00Z"))
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 reservation (one user), got %d", len(res))
	}

	if res[0].Resources["nvidia.com/gpu"] != 1 {
		t.Errorf("nvidia.com/gpu = %d, want 1", res[0].Resources["nvidia.com/gpu"])
	}
	if res[0].Resources["nvidia.com/mig-3g.71gb"] != 1 {
		t.Errorf("nvidia.com/mig-3g.71gb = %d, want 1", res[0].Resources["nvidia.com/mig-3g.71gb"])
	}
	// Unreserved types still included with count 0
	if _, ok := res[0].Resources["nvidia.com/mig-2g.35gb"]; !ok {
		t.Error("nvidia.com/mig-2g.35gb missing from Resources — all GPU types must be included")
	}
	if _, ok := res[0].Resources["nvidia.com/mig-1g.18gb"]; !ok {
		t.Error("nvidia.com/mig-1g.18gb missing from Resources — all GPU types must be included")
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return parsed
}
