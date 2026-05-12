package kube

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eformat/gpu-booking-plugin/pkg/database"
)

var (
	DiscoveryEnabled  bool
	DiscoveryInterval int // seconds between re-discoveries
)

type discoveredNode struct {
	Name          string
	GPUProduct    string
	GPUMemoryMiB  int
	GPUCount      int
	MIGStrategy   string
	Labels        map[string]string
	Allocatable   map[string]string
	AllocCPU      int
	AllocMemoryGi int
}

type discoveredFlavor struct {
	Name       string
	NodeLabels map[string]string
	Labels     map[string]string
}

type DiscoveryResult struct {
	Resources   []database.GPUResourceSpec
	TotalCPU    int
	TotalMemory int
	FlavorName  string
}

func InitDiscovery() {
	if !DiscoveryEnabled {
		slog.Info("gpu discovery: disabled")
		return
	}
	if k8sHost == "" || k8sToken == "" {
		slog.Info("gpu discovery: no k8s client, skipping")
		return
	}

	result, err := RunDiscovery()
	if err != nil {
		slog.Error("gpu discovery: initial discovery failed, using static config", "error", err)
	} else if result != nil {
		applyDiscoveryResult(result)
		slog.Info("gpu discovery: initial config applied",
			"resources", len(result.Resources),
			"totalCPU", result.TotalCPU,
			"totalMemory", result.TotalMemory,
			"flavorName", result.FlavorName)
	} else {
		slog.Info("gpu discovery: no GPU nodes found, keeping static config")
	}

	go discoveryLoop()
}

func applyDiscoveryResult(result *DiscoveryResult) {
	database.SetGPUConfig(&database.GPUConfig{
		Resources:   result.Resources,
		TotalCPU:    result.TotalCPU,
		TotalMemory: result.TotalMemory,
		FlavorName:  result.FlavorName,
	})
}

func discoveryLoop() {
	interval := DiscoveryInterval
	if interval <= 0 {
		interval = 300
	}
	for {
		time.Sleep(time.Duration(interval) * time.Second)
		result, err := RunDiscovery()
		if err != nil {
			slog.Error("gpu discovery: periodic discovery failed", "error", err)
			continue
		}
		if result != nil {
			applyDiscoveryResult(result)
			slog.Info("gpu discovery: config updated",
				"resources", len(result.Resources),
				"flavorName", result.FlavorName)
		}
	}
}

func RunDiscovery() (*DiscoveryResult, error) {
	nodes, err := discoverGPUNodes()
	if err != nil {
		return nil, fmt.Errorf("discovering GPU nodes: %w", err)
	}
	if len(nodes) == 0 {
		return nil, nil
	}

	flavors, _ := discoverResourceFlavors()

	flavorName := matchFlavorToNodes(flavors, nodes)

	var fullGPUMemoryMiB int
	for _, n := range nodes {
		if n.GPUMemoryMiB > 0 {
			fullGPUMemoryMiB = n.GPUMemoryMiB
			break
		}
	}

	var gpuProduct string
	for _, n := range nodes {
		if n.GPUProduct != "" {
			gpuProduct = n.GPUProduct
			break
		}
	}

	resources := buildResourceSpecs(nodes, fullGPUMemoryMiB, gpuProduct)
	totalCPU, totalMemGi := computeNodeTotals(nodes)

	return &DiscoveryResult{
		Resources:   resources,
		TotalCPU:    totalCPU,
		TotalMemory: totalMemGi,
		FlavorName:  flavorName,
	}, nil
}

func discoverGPUNodes() ([]discoveredNode, error) {
	body, err := K8sGet("/api/v1/nodes?labelSelector=nvidia.com%2Fgpu.present%3Dtrue")
	if err != nil {
		return nil, fmt.Errorf("listing GPU nodes: %w", err)
	}

	var nodeList struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Allocatable map[string]string `json:"allocatable"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &nodeList); err != nil {
		return nil, fmt.Errorf("parsing node list: %w", err)
	}

	var nodes []discoveredNode
	for _, item := range nodeList.Items {
		n := discoveredNode{
			Name:        item.Metadata.Name,
			GPUProduct:  item.Metadata.Labels["nvidia.com/gpu.product"],
			MIGStrategy: item.Metadata.Labels["nvidia.com/mig.strategy"],
			Labels:      item.Metadata.Labels,
			Allocatable: item.Status.Allocatable,
		}

		if memStr := item.Metadata.Labels["nvidia.com/gpu.memory"]; memStr != "" {
			n.GPUMemoryMiB, _ = strconv.Atoi(memStr)
		}
		if countStr := item.Metadata.Labels["nvidia.com/gpu.count"]; countStr != "" {
			n.GPUCount, _ = strconv.Atoi(countStr)
		}

		n.AllocCPU = parseCPU(item.Status.Allocatable["cpu"])
		n.AllocMemoryGi = parseMemoryGi(item.Status.Allocatable["memory"])

		nodes = append(nodes, n)
	}

	return nodes, nil
}

func discoverResourceFlavors() ([]discoveredFlavor, error) {
	body, err := K8sGet("/apis/kueue.x-k8s.io/v1beta1/resourceflavors")
	if err != nil {
		return nil, nil
	}

	var flavorList struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				NodeLabels map[string]string `json:"nodeLabels"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &flavorList); err != nil {
		return nil, nil
	}

	var flavors []discoveredFlavor
	for _, item := range flavorList.Items {
		flavors = append(flavors, discoveredFlavor{
			Name:       item.Metadata.Name,
			NodeLabels: item.Spec.NodeLabels,
			Labels:     item.Metadata.Labels,
		})
	}
	return flavors, nil
}

func matchFlavorToNodes(flavors []discoveredFlavor, nodes []discoveredNode) string {
	if len(nodes) == 0 {
		return ""
	}

	var wildcard string
	for _, f := range flavors {
		if f.Name == "default-flavor" {
			continue
		}
		if len(f.NodeLabels) == 0 {
			if wildcard == "" || f.Labels["gpu-config-plugin.openshift.io/managed"] == "true" {
				wildcard = f.Name
			}
			continue
		}
		matched := true
		for k, v := range f.NodeLabels {
			found := false
			for _, n := range nodes {
				if nv, ok := n.Allocatable[k]; ok && nv == v {
					found = true
					break
				}
				if nodeLabel, ok := getNodeLabel(n, k); ok && nodeLabel == v {
					found = true
					break
				}
			}
			if !found {
				matched = false
				break
			}
		}
		if matched {
			return f.Name
		}
	}
	return wildcard
}

func getNodeLabel(n discoveredNode, key string) (string, bool) {
	switch key {
	case "nvidia.com/gpu.product":
		return n.GPUProduct, n.GPUProduct != ""
	case "nvidia.com/gpu.memory":
		if n.GPUMemoryMiB > 0 {
			return strconv.Itoa(n.GPUMemoryMiB), true
		}
		return "", false
	default:
		if v, ok := n.Labels[key]; ok {
			return v, true
		}
		return "", false
	}
}

var migResourceRegex = regexp.MustCompile(`^nvidia\.com/mig-(\d+g)\.(\d+)gb$`)

func buildResourceSpecs(nodes []discoveredNode, fullGPUMemoryMiB int, gpuProduct string) []database.GPUResourceSpec {
	type resInfo struct {
		count    int
		memoryGB int
	}

	totalFullGPU := 0
	migResources := map[string]*resInfo{}

	for _, n := range nodes {
		for key, valStr := range n.Allocatable {
			count, err := strconv.Atoi(valStr)
			if err != nil || count <= 0 {
				continue
			}

			if key == "nvidia.com/gpu" {
				totalFullGPU += count
				continue
			}

			if m := migResourceRegex.FindStringSubmatch(key); m != nil {
				memGB, _ := strconv.Atoi(m[2])
				if existing, ok := migResources[key]; ok {
					existing.count += count
				} else {
					migResources[key] = &resInfo{count: count, memoryGB: memGB}
				}
			}
		}
	}

	if totalFullGPU == 0 && len(migResources) == 0 {
		return nil
	}

	gpuName := formatGPUProductName(gpuProduct)

	var specs []database.GPUResourceSpec

	if totalFullGPU > 0 {
		specs = append(specs, database.GPUResourceSpec{
			Name:          gpuName + " Full GPU",
			Type:          "nvidia.com/gpu",
			Count:         totalFullGPU,
			GPUEquivalent: 1.0,
		})
	}

	var migKeys []string
	for k := range migResources {
		migKeys = append(migKeys, k)
	}
	sort.Slice(migKeys, func(i, j int) bool {
		return migResources[migKeys[i]].memoryGB > migResources[migKeys[j]].memoryGB
	})

	fullMemGB := float64(fullGPUMemoryMiB) / 1024.0
	if fullMemGB <= 0 {
		fullMemGB = 1.0
	}

	for _, key := range migKeys {
		info := migResources[key]
		parts := strings.TrimPrefix(key, "nvidia.com/")
		gpuEquiv := float64(info.memoryGB) / fullMemGB

		specs = append(specs, database.GPUResourceSpec{
			Name:          "MIG " + strings.TrimPrefix(parts, "mig-"),
			Type:          key,
			Count:         info.count,
			GPUEquivalent: math.Round(gpuEquiv*1000) / 1000,
		})
	}

	totalEquivUnits := 0.0
	for i := range specs {
		totalEquivUnits += float64(specs[i].Count) * specs[i].GPUEquivalent
	}
	if totalEquivUnits > 0 {
		for i := range specs {
			specs[i].Share = specs[i].GPUEquivalent / totalEquivUnits
		}
	}

	return specs
}

func formatGPUProductName(product string) string {
	if product == "" {
		return "GPU"
	}
	return strings.ReplaceAll(product, "-", " ")
}

func computeNodeTotals(nodes []discoveredNode) (totalCPU, totalMemGi int) {
	for _, n := range nodes {
		totalCPU += n.AllocCPU
		totalMemGi += n.AllocMemoryGi
	}
	return
}

func parseCPU(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if millis, ok := strings.CutSuffix(s, "m"); ok {
		m, _ := strconv.Atoi(millis)
		return m / 1000
	}
	v, _ := strconv.Atoi(s)
	return v
}

func parseMemoryGi(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if val, ok := strings.CutSuffix(s, "Ki"); ok {
		v, _ := strconv.ParseInt(val, 10, 64)
		return int(v / (1024 * 1024))
	}
	if val, ok := strings.CutSuffix(s, "Mi"); ok {
		v, _ := strconv.ParseInt(val, 10, 64)
		return int(v / 1024)
	}
	if val, ok := strings.CutSuffix(s, "Gi"); ok {
		v, _ := strconv.Atoi(val)
		return v
	}
	if val, ok := strings.CutSuffix(s, "Ti"); ok {
		v, _ := strconv.Atoi(val)
		return v * 1024
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return int(v / (1024 * 1024 * 1024))
}
