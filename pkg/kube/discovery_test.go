package kube

import (
	"testing"
)

func TestBuildResourceSpecs_FullGPUOnly(t *testing.T) {
	nodes := []discoveredNode{
		{
			Name:         "node-1",
			GPUProduct:   "NVIDIA-A100-SXM4-40GB",
			GPUMemoryMiB: 40960,
			GPUCount:     8,
			Allocatable: map[string]string{
				"nvidia.com/gpu": "8",
				"cpu":            "128",
				"memory":         "512Gi",
			},
		},
	}

	specs := buildResourceSpecs(nodes, 40960, "NVIDIA-A100-SXM4-40GB")
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}

	s := specs[0]
	if s.Name != "NVIDIA A100 SXM4 40GB Full GPU" {
		t.Errorf("Name = %q", s.Name)
	}
	if s.Type != "nvidia.com/gpu" {
		t.Errorf("Type = %q", s.Type)
	}
	if s.Count != 8 {
		t.Errorf("Count = %d, want 8", s.Count)
	}
	if s.GPUEquivalent != 1.0 {
		t.Errorf("GPUEquivalent = %f, want 1.0", s.GPUEquivalent)
	}
	if s.Share == 0 {
		t.Error("Share should be non-zero")
	}
}

func TestBuildResourceSpecs_WithMIG(t *testing.T) {
	nodes := []discoveredNode{
		{
			Name:         "node-1",
			GPUProduct:   "NVIDIA-H200",
			GPUMemoryMiB: 143360, // ~140 GiB
			GPUCount:     8,
			MIGStrategy:  "mixed",
			Allocatable: map[string]string{
				"nvidia.com/gpu":          "8",
				"nvidia.com/mig-3g.71gb":  "8",
				"nvidia.com/mig-2g.35gb":  "8",
				"nvidia.com/mig-1g.18gb":  "16",
				"cpu":                     "128",
				"memory":                  "512Gi",
			},
		},
	}

	specs := buildResourceSpecs(nodes, 143360, "NVIDIA-H200")
	if len(specs) != 4 {
		t.Fatalf("expected 4 specs, got %d: %+v", len(specs), specs)
	}

	// First should be full GPU
	if specs[0].Type != "nvidia.com/gpu" {
		t.Errorf("first spec should be full GPU, got %q", specs[0].Type)
	}
	if specs[0].Count != 8 {
		t.Errorf("full GPU count = %d, want 8", specs[0].Count)
	}

	// MIG specs sorted by memory descending
	if specs[1].Type != "nvidia.com/mig-3g.71gb" {
		t.Errorf("second spec should be mig-3g.71gb, got %q", specs[1].Type)
	}
	if specs[1].Name != "MIG 3g.71gb" {
		t.Errorf("MIG name = %q, want 'MIG 3g.71gb'", specs[1].Name)
	}
	if specs[2].Type != "nvidia.com/mig-2g.35gb" {
		t.Errorf("third spec should be mig-2g.35gb, got %q", specs[2].Type)
	}
	if specs[3].Type != "nvidia.com/mig-1g.18gb" {
		t.Errorf("fourth spec should be mig-1g.18gb, got %q", specs[3].Type)
	}

	// Verify gpuEquivalent ratios
	if specs[1].GPUEquivalent <= 0 || specs[1].GPUEquivalent >= 1.0 {
		t.Errorf("MIG 71gb gpuEquivalent should be between 0 and 1, got %f", specs[1].GPUEquivalent)
	}
	if specs[1].GPUEquivalent <= specs[2].GPUEquivalent {
		t.Errorf("MIG 71gb gpuEquiv (%f) should be > MIG 35gb gpuEquiv (%f)",
			specs[1].GPUEquivalent, specs[2].GPUEquivalent)
	}

	// Verify shares sum to ~1.0
	totalShare := 0.0
	totalEquivUnits := 0.0
	for _, s := range specs {
		totalShare += float64(s.Count) * s.Share
		totalEquivUnits += float64(s.Count) * s.GPUEquivalent
	}
	// Each spec.Share = spec.GPUEquivalent / totalEquivUnits
	// So sum(count * share) = sum(count * gpuEquiv) / totalEquivUnits = 1.0
	if totalShare < 0.99 || totalShare > 1.01 {
		t.Errorf("total share = %f, expected ~1.0", totalShare)
	}
}

func TestBuildResourceSpecs_MultiNode(t *testing.T) {
	nodes := []discoveredNode{
		{
			Name:         "node-1",
			GPUProduct:   "NVIDIA-A100-SXM4-40GB",
			GPUMemoryMiB: 40960,
			GPUCount:     4,
			Allocatable: map[string]string{
				"nvidia.com/gpu": "4",
				"cpu":            "64",
				"memory":         "256Gi",
			},
		},
		{
			Name:         "node-2",
			GPUProduct:   "NVIDIA-A100-SXM4-40GB",
			GPUMemoryMiB: 40960,
			GPUCount:     4,
			Allocatable: map[string]string{
				"nvidia.com/gpu": "4",
				"cpu":            "64",
				"memory":         "256Gi",
			},
		},
	}

	specs := buildResourceSpecs(nodes, 40960, "NVIDIA-A100-SXM4-40GB")
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].Count != 8 {
		t.Errorf("Count = %d, want 8 (aggregated from 2 nodes)", specs[0].Count)
	}
}

func TestBuildResourceSpecs_NoGPUs(t *testing.T) {
	nodes := []discoveredNode{
		{
			Name: "node-1",
			Allocatable: map[string]string{
				"cpu":    "64",
				"memory": "256Gi",
			},
		},
	}

	specs := buildResourceSpecs(nodes, 0, "")
	if specs != nil {
		t.Errorf("expected nil specs for no GPUs, got %+v", specs)
	}
}

func TestBuildResourceSpecs_MultiNodeWithMIG(t *testing.T) {
	nodes := []discoveredNode{
		{
			Name:         "node-1",
			GPUProduct:   "NVIDIA-H100-80GB-HBM3",
			GPUMemoryMiB: 81920,
			GPUCount:     8,
			MIGStrategy:  "mixed",
			Allocatable: map[string]string{
				"nvidia.com/gpu":         "8",
				"nvidia.com/mig-3g.40gb": "16",
				"nvidia.com/mig-1g.10gb": "56",
			},
		},
		{
			Name:         "node-2",
			GPUProduct:   "NVIDIA-H100-80GB-HBM3",
			GPUMemoryMiB: 81920,
			GPUCount:     8,
			MIGStrategy:  "mixed",
			Allocatable: map[string]string{
				"nvidia.com/gpu":         "8",
				"nvidia.com/mig-3g.40gb": "16",
				"nvidia.com/mig-1g.10gb": "56",
			},
		},
	}

	specs := buildResourceSpecs(nodes, 81920, "NVIDIA-H100-80GB-HBM3")
	if len(specs) != 3 {
		t.Fatalf("expected 3 specs, got %d: %+v", len(specs), specs)
	}

	// Full GPUs aggregated across nodes
	if specs[0].Type != "nvidia.com/gpu" || specs[0].Count != 16 {
		t.Errorf("full GPU: type=%q count=%d, want nvidia.com/gpu count=16", specs[0].Type, specs[0].Count)
	}

	// MIG aggregated across nodes
	for _, s := range specs[1:] {
		if s.Type == "nvidia.com/mig-3g.40gb" && s.Count != 32 {
			t.Errorf("mig-3g.40gb count=%d, want 32", s.Count)
		}
		if s.Type == "nvidia.com/mig-1g.10gb" && s.Count != 112 {
			t.Errorf("mig-1g.10gb count=%d, want 112", s.Count)
		}
	}
}

func TestBuildResourceSpecs_ZeroedAllocatable(t *testing.T) {
	nodes := []discoveredNode{
		{
			Name:         "node-1",
			GPUProduct:   "NVIDIA-L40S",
			GPUMemoryMiB: 49152,
			GPUCount:     8,
			Allocatable: map[string]string{
				"nvidia.com/gpu":         "8",
				"nvidia.com/mig-1g.18gb": "0",
				"nvidia.com/mig-3g.71gb": "0",
			},
		},
	}

	specs := buildResourceSpecs(nodes, 49152, "NVIDIA-L40S")
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec (zeroed MIG should be skipped), got %d: %+v", len(specs), specs)
	}
	if specs[0].Type != "nvidia.com/gpu" {
		t.Errorf("expected nvidia.com/gpu, got %q", specs[0].Type)
	}
}

func TestComputeNodeTotals(t *testing.T) {
	nodes := []discoveredNode{
		{AllocCPU: 64, AllocMemoryGi: 256},
		{AllocCPU: 128, AllocMemoryGi: 512},
	}

	cpu, mem := computeNodeTotals(nodes)
	if cpu != 192 {
		t.Errorf("CPU = %d, want 192", cpu)
	}
	if mem != 768 {
		t.Errorf("Memory = %d, want 768", mem)
	}
}

func TestParseCPU(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"128", 128},
		{"64000m", 64},
		{"", 0},
		{"2", 2},
	}
	for _, tt := range tests {
		got := parseCPU(tt.input)
		if got != tt.want {
			t.Errorf("parseCPU(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseMemoryGi(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"512Gi", 512},
		{"524288Mi", 512},
		{"536870912Ki", 512},
		{"549755813888", 512},
		{"1Ti", 1024},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseMemoryGi(tt.input)
		if got != tt.want {
			t.Errorf("parseMemoryGi(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFormatGPUProductName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"NVIDIA-A100-SXM4-40GB", "NVIDIA A100 SXM4 40GB"},
		{"NVIDIA-H200", "NVIDIA H200"},
		{"", "GPU"},
	}
	for _, tt := range tests {
		got := formatGPUProductName(tt.input)
		if got != tt.want {
			t.Errorf("formatGPUProductName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMatchFlavorToNodes(t *testing.T) {
	nodes := []discoveredNode{
		{
			GPUProduct:   "NVIDIA-H200",
			GPUMemoryMiB: 143360,
			Allocatable: map[string]string{
				"nvidia.com/gpu": "8",
			},
		},
	}

	flavors := []discoveredFlavor{
		{
			Name: "default-flavor",
			NodeLabels: map[string]string{
				"some-other-label": "value",
			},
		},
		{
			Name: "h200",
			NodeLabels: map[string]string{
				"nvidia.com/gpu.product": "NVIDIA-H200",
			},
		},
	}

	name := matchFlavorToNodes(flavors, nodes)
	if name != "h200" {
		t.Errorf("matchFlavorToNodes = %q, want 'h200'", name)
	}
}

func TestMatchFlavorToNodes_NoMatch(t *testing.T) {
	nodes := []discoveredNode{
		{GPUProduct: "NVIDIA-A100"},
	}
	flavors := []discoveredFlavor{
		{
			Name:       "h200",
			NodeLabels: map[string]string{"nvidia.com/gpu.product": "NVIDIA-H200"},
		},
	}
	name := matchFlavorToNodes(flavors, nodes)
	if name != "" {
		t.Errorf("expected empty, got %q", name)
	}
}

func TestMatchFlavorToNodes_EmptyFlavors(t *testing.T) {
	nodes := []discoveredNode{{GPUProduct: "NVIDIA-A100"}}
	name := matchFlavorToNodes(nil, nodes)
	if name != "" {
		t.Errorf("expected empty, got %q", name)
	}
}
