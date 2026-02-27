/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_StandardScheduler(t *testing.T) {
	testContent := `
# Standard scheduler config without TAS
cohorts:
  - className: cohort
    count: 2
    queuesSets:
      - className: cq
        count: 3
        nominalQuota: 20
        borrowingLimit: 100
        reclaimWithinCohort: Any
        withinClusterQueue: LowerPriority
        workloadsSets:
          - count: 100
            creationIntervalMs: 100
            workloads:
              - className: small
                runtimeMs: 200
                priority: 50
                request: 1
              - className: medium
                runtimeMs: 500
                priority: 100
                request: 5
`
	tempDir := t.TempDir()
	fPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(fPath, []byte(testContent), os.FileMode(0600)); err != nil {
		t.Fatalf("unable to create test file: %v", err)
	}

	got, err := LoadConfig(fPath)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	// Should have cohorts
	if len(got.Cohorts) != 1 {
		t.Errorf("expected 1 cohort, got %d", len(got.Cohorts))
	}

	// Should have default resource flavor
	if got.ResourceFlavor == nil {
		t.Fatal("expected default resource flavor to be set")
	}
	if got.ResourceFlavor.Name != "rf" {
		t.Errorf("expected default resource flavor name 'rf', got %q", got.ResourceFlavor.Name)
	}

	// Should not have topology (standard mode)
	if got.Topology != nil {
		t.Error("expected no topology for standard scheduler config")
	}

	// Verify cohort structure
	cohort := got.Cohorts[0]
	if cohort.ClassName != "cohort" {
		t.Errorf("expected className 'cohort', got %q", cohort.ClassName)
	}
	if cohort.Count != 2 {
		t.Errorf("expected count 2, got %d", cohort.Count)
	}
	if len(cohort.QueuesSets) != 1 {
		t.Errorf("expected 1 queueSet, got %d", len(cohort.QueuesSets))
	}

	// Verify queue structure
	queue := cohort.QueuesSets[0]
	if queue.ClassName != "cq" {
		t.Errorf("expected className 'cq', got %q", queue.ClassName)
	}
	if queue.Count != 3 {
		t.Errorf("expected count 3, got %d", queue.Count)
	}
	if queue.NominalQuota != "20" {
		t.Errorf("expected nominalQuota '20', got %q", queue.NominalQuota)
	}

	// Verify workloads
	if len(queue.WorkloadsSets) != 1 {
		t.Errorf("expected 1 workloadsSet, got %d", len(queue.WorkloadsSets))
	}
	wlSet := queue.WorkloadsSets[0]
	if wlSet.Count != 100 {
		t.Errorf("expected count 100, got %d", wlSet.Count)
	}
	if len(wlSet.Workloads) != 2 {
		t.Errorf("expected 2 workload templates, got %d", len(wlSet.Workloads))
	}
}

func TestLoadConfig_TAS(t *testing.T) {
	testContent := `
# TAS config with topology
topology:
  name: default-topology
  levels:
    - name: block
      count: 1
      nodeLabel: "topology.kubernetes.io/block"
    - name: rack
      count: 10
      nodeLabel: "topology.kubernetes.io/rack"
    - name: node
      count: 64
      nodeLabel: "kubernetes.io/hostname"
      capacity:
        cpu: "96"
        memory: "256Gi"

resourceFlavor:
  name: tas-flavor
  nodeLabel: "tas-node-group"
  topologyName: "default-topology"

cohorts:
  - className: tas-cohort
    count: 1
    queuesSets:
      - className: tas-cq
        count: 2
        nominalQuota: 50
        borrowingLimit: 200
        reclaimWithinCohort: Any
        withinClusterQueue: LowerPriority
        workloadsSets:
          - count: 50
            creationIntervalMs: 200
            workloads:
              - className: tas-workload
                runtimeMs: 1000
                priority: 100
                request: 2
                podCount: 8
                tasConstraint: required
                tasLevel: rack
`
	tempDir := t.TempDir()
	fPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(fPath, []byte(testContent), os.FileMode(0600)); err != nil {
		t.Fatalf("unable to create test file: %v", err)
	}

	got, err := LoadConfig(fPath)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	// Should have topology
	if got.Topology == nil {
		t.Fatal("expected topology to be set")
	}
	if got.Topology.Name != "default-topology" {
		t.Errorf("expected topology name 'default-topology', got %q", got.Topology.Name)
	}
	if len(got.Topology.Levels) != 3 {
		t.Errorf("expected 3 topology levels, got %d", len(got.Topology.Levels))
	}

	// Should have custom resource flavor
	if got.ResourceFlavor == nil {
		t.Fatal("expected resource flavor to be set")
	}
	if got.ResourceFlavor.Name != "tas-flavor" {
		t.Errorf("expected resource flavor name 'tas-flavor', got %q", got.ResourceFlavor.Name)
	}
	if got.ResourceFlavor.NodeLabel != "tas-node-group" {
		t.Errorf("expected node label 'tas-node-group', got %q", got.ResourceFlavor.NodeLabel)
	}

	// Should have cohorts
	if len(got.Cohorts) != 1 {
		t.Errorf("expected 1 cohort, got %d", len(got.Cohorts))
	}

	// Verify TAS workload template
	cohort := got.Cohorts[0]
	queue := cohort.QueuesSets[0]
	wlSet := queue.WorkloadsSets[0]
	if len(wlSet.Workloads) != 1 {
		t.Fatalf("expected 1 workload template, got %d", len(wlSet.Workloads))
	}

	wl := wlSet.Workloads[0]
	if wl.PodCount != 8 {
		t.Errorf("expected podCount 8, got %d", wl.PodCount)
	}
	if wl.TASConstraint != "required" {
		t.Errorf("expected tasConstraint 'required', got %q", wl.TASConstraint)
	}
	if wl.TASLevel != "rack" {
		t.Errorf("expected tasLevel 'rack', got %q", wl.TASLevel)
	}
}

func TestLoadConfig_EmptyCohorts(t *testing.T) {
	testContent := `
# Config with no cohorts
cohorts: []
`
	tempDir := t.TempDir()
	fPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(fPath, []byte(testContent), os.FileMode(0600)); err != nil {
		t.Fatalf("unable to create test file: %v", err)
	}

	_, err := LoadConfig(fPath)
	if err == nil {
		t.Fatal("expected error for empty cohorts, got nil")
	}
	if err.Error() != "config must contain at least one cohort" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	testContent := `
cohorts:
  - className: invalid
    count: not-a-number
`
	tempDir := t.TempDir()
	fPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(fPath, []byte(testContent), os.FileMode(0600)); err != nil {
		t.Fatalf("unable to create test file: %v", err)
	}

	_, err := LoadConfig(fPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadConfig_TASBalancedPlacement(t *testing.T) {
	testContent := `
topology:
  name: test-topology
  levels:
    - name: node
      count: 16
      nodeLabel: "kubernetes.io/hostname"
      capacity:
        cpu: "96"
        memory: "256Gi"

resourceFlavor:
  name: test-flavor
  nodeLabel: "tas-node-group"
  topologyName: "default-topology"

cohorts:
  - className: balanced-cohort
    count: 1
    queuesSets:
      - className: balanced-cq
        count: 1
        nominalQuota: 100
        borrowingLimit: 0
        reclaimWithinCohort: Never
        withinClusterQueue: Never
        workloadsSets:
          - count: 10
            creationIntervalMs: 100
            workloads:
              - className: balanced-wl
                runtimeMs: 500
                priority: 100
                request: 1
                podCount: 16
                tasConstraint: balanced
                tasLevel: node
                sliceSize: 4
`
	tempDir := t.TempDir()
	fPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(fPath, []byte(testContent), os.FileMode(0600)); err != nil {
		t.Fatalf("unable to create test file: %v", err)
	}

	got, err := LoadConfig(fPath)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	// Verify balanced placement parameters
	wl := got.Cohorts[0].QueuesSets[0].WorkloadsSets[0].Workloads[0]
	if wl.TASConstraint != "balanced" {
		t.Errorf("expected tasConstraint 'balanced', got %q", wl.TASConstraint)
	}
	if wl.SliceSize != 4 {
		t.Errorf("expected sliceSize 4, got %d", wl.SliceSize)
	}
	if wl.PodCount != 16 {
		t.Errorf("expected podCount 16, got %d", wl.PodCount)
	}
}

func TestLoadConfig_UnschedulableScenario(t *testing.T) {
	testContent := `
cohorts:
  - className: cohort
    count: 1
    queuesSets:
      - className: borrower-cq
        count: 1
        resources:
          - name: pods
            nominalQuota: 1000
          - name: cpu
            nominalQuota: 0
            borrowingLimit: 100
        reclaimWithinCohort: Any
        withinClusterQueue: LowerPriority
        workloadsSets:
          - count: 200
            creationIntervalMs: 50
            workloads:
              - className: needs-cpu
                runtimeMs: 500
                priority: 200
                request: 200m
          - count: 100
            creationIntervalMs: 100
            workloads:
              - className: pods-only
                runtimeMs: 5000
                priority: 50
      - className: capacity-cq
        count: 1
        resources:
          - name: cpu
            nominalQuota: 100
        reclaimWithinCohort: Any
        withinClusterQueue: LowerPriority
        workloadsSets:
          - count: 500
            creationIntervalMs: 10
            workloads:
              - className: cpu-hog
                runtimeMs: 60000
                priority: 500
                request: 200m
`
	tempDir := t.TempDir()
	fPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(fPath, []byte(testContent), os.FileMode(0600)); err != nil {
		t.Fatalf("unable to create test file: %v", err)
	}

	got, err := LoadConfig(fPath)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	if len(got.Cohorts) != 1 {
		t.Fatalf("expected 1 cohort, got %d", len(got.Cohorts))
	}

	cohort := got.Cohorts[0]
	if len(cohort.QueuesSets) != 2 {
		t.Fatalf("expected 2 queueSets, got %d", len(cohort.QueuesSets))
	}

	// Verify borrower CQ with multi-resource config
	borrower := cohort.QueuesSets[0]
	if borrower.ClassName != "borrower-cq" {
		t.Errorf("expected className 'borrower-cq', got %q", borrower.ClassName)
	}
	if len(borrower.Resources) != 2 {
		t.Fatalf("expected 2 resources in borrower, got %d", len(borrower.Resources))
	}
	if borrower.Resources[0].Name != "pods" {
		t.Errorf("expected first resource 'pods', got %q", borrower.Resources[0].Name)
	}
	if borrower.Resources[0].NominalQuota != "1000" {
		t.Errorf("expected pods nominalQuota '1000', got %q", borrower.Resources[0].NominalQuota)
	}
	if borrower.Resources[1].Name != "cpu" {
		t.Errorf("expected second resource 'cpu', got %q", borrower.Resources[1].Name)
	}
	if borrower.Resources[1].BorrowingLimit != "100" {
		t.Errorf("expected cpu borrowingLimit '100', got %q", borrower.Resources[1].BorrowingLimit)
	}
	if len(borrower.WorkloadsSets) != 2 {
		t.Fatalf("expected 2 workloadsSets in borrower, got %d", len(borrower.WorkloadsSets))
	}

	// Inadmissible workloads: high priority, request CPU
	needsCPU := borrower.WorkloadsSets[0].Workloads[0]
	if needsCPU.ClassName != "needs-cpu" {
		t.Errorf("expected className 'needs-cpu', got %q", needsCPU.ClassName)
	}
	if needsCPU.Priority != 200 {
		t.Errorf("expected priority 200, got %d", needsCPU.Priority)
	}
	if needsCPU.Request != "200m" {
		t.Errorf("expected request '200m', got %q", needsCPU.Request)
	}

	// Admissible workloads: lower priority, no CPU request
	podsOnly := borrower.WorkloadsSets[1].Workloads[0]
	if podsOnly.ClassName != "pods-only" {
		t.Errorf("expected className 'pods-only', got %q", podsOnly.ClassName)
	}
	if podsOnly.Priority != 50 {
		t.Errorf("expected priority 50, got %d", podsOnly.Priority)
	}
	if podsOnly.Request != "" {
		t.Errorf("expected empty request for pods-only, got %q", podsOnly.Request)
	}

	// Verify capacity CQ
	capacity := cohort.QueuesSets[1]
	if capacity.ClassName != "capacity-cq" {
		t.Errorf("expected className 'capacity-cq', got %q", capacity.ClassName)
	}
	if len(capacity.Resources) != 1 {
		t.Fatalf("expected 1 resource in capacity, got %d", len(capacity.Resources))
	}
	if capacity.Resources[0].NominalQuota != "100" {
		t.Errorf("expected nominalQuota '100', got %q", capacity.Resources[0].NominalQuota)
	}
}
