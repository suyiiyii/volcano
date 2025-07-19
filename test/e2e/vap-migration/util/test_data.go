/*
Copyright 2024 The Volcano Authors.

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

package util

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// JobTestScenario represents a single job validation test scenario
type JobTestScenario struct {
	Name     string                `yaml:"name"`
	JobSpec  *v1alpha1.JobSpec     `yaml:"spec"`
	Expected ExpectedValidationResult `yaml:"expected_result"`
}

// ExpectedValidationResult represents the expected outcome of validation
type ExpectedValidationResult struct {
	Allowed       bool   `yaml:"allowed"`
	ErrorContains string `yaml:"error_contains,omitempty"`
}

// JobTestScenarios contains all job test scenarios organized by category
type JobTestScenarios struct {
	BasicFieldValidation     []JobTestScenario `yaml:"basic_field_validation"`
	CrossFieldValidation     []JobTestScenario `yaml:"cross_field_validation"`
	TaskStructureValidation  []JobTestScenario `yaml:"task_structure_validation"`
	QueueValidation          []JobTestScenario `yaml:"queue_validation"`
	PluginValidation         []JobTestScenario `yaml:"plugin_validation"`
	ResourceValidation       []JobTestScenario `yaml:"resource_validation"`
}

// LoadJobTestScenarios loads job test scenarios from embedded data
func LoadJobTestScenarios() *JobTestScenarios {
	// For now, create programmatically. In a real implementation, 
	// this would load from YAML files
	return &JobTestScenarios{
		BasicFieldValidation: []JobTestScenario{
			{
				Name: "valid_basic_job",
				JobSpec: &v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task1",
							Replicas: 2,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "nginx",
											Image: "nginx:1.14",
										},
									},
								},
							},
						},
					},
				},
				Expected: ExpectedValidationResult{
					Allowed: true,
				},
			},
			{
				Name: "invalid_minAvailable_negative",
				JobSpec: &v1alpha1.JobSpec{
					MinAvailable: -1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task1",
							Replicas: 1,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "nginx",
											Image: "nginx:1.14",
										},
									},
								},
							},
						},
					},
				},
				Expected: ExpectedValidationResult{
					Allowed:       false,
					ErrorContains: "minAvailable' must be >= 0",
				},
			},
			{
				Name: "invalid_maxRetry_negative",
				JobSpec: &v1alpha1.JobSpec{
					MinAvailable: 1,
					MaxRetry:     -1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task1",
							Replicas: 1,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "nginx",
											Image: "nginx:1.14",
										},
									},
								},
							},
						},
					},
				},
				Expected: ExpectedValidationResult{
					Allowed:       false,
					ErrorContains: "'maxRetry' cannot be less than zero",
				},
			},
		},
		CrossFieldValidation: []JobTestScenario{
			{
				Name: "minAvailable_exceeds_total_replicas",
				JobSpec: &v1alpha1.JobSpec{
					MinAvailable: 5,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task1",
							Replicas: 2,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "nginx",
											Image: "nginx:1.14",
										},
									},
								},
							},
						},
					},
				},
				Expected: ExpectedValidationResult{
					Allowed:       false,
					ErrorContains: "minAvailable' should not be greater than total replicas",
				},
			},
		},
		TaskStructureValidation: []JobTestScenario{
			{
				Name: "no_tasks_defined",
				JobSpec: &v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks:        []v1alpha1.TaskSpec{},
				},
				Expected: ExpectedValidationResult{
					Allowed:       false,
					ErrorContains: "No task specified in job spec",
				},
			},
			{
				Name: "duplicate_task_names",
				JobSpec: &v1alpha1.JobSpec{
					MinAvailable: 1,
					Queue:        "default",
					Tasks: []v1alpha1.TaskSpec{
						{
							Name:     "task1",
							Replicas: 1,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Name: "nginx", Image: "nginx:1.14"},
									},
								},
							},
						},
						{
							Name:     "task1", // duplicate name
							Replicas: 1,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Name: "nginx", Image: "nginx:1.14"},
									},
								},
							},
						},
					},
				},
				Expected: ExpectedValidationResult{
					Allowed:       false,
					ErrorContains: "duplicated task name",
				},
			},
		},
	}
}

// LoadJobEdgeCases loads edge case scenarios for comprehensive testing
func LoadJobEdgeCases() []JobTestScenario {
	return []JobTestScenario{
		{
			Name: "maximum_task_count",
			JobSpec: &v1alpha1.JobSpec{
				MinAvailable: 50,
				Queue:        "default",
				Tasks:        generateMaxTaskSpecs(50), // Generate many tasks
			},
			Expected: ExpectedValidationResult{
				Allowed: true,
			},
		},
	}
}

// generateMaxTaskSpecs generates a specified number of task specs for testing
func generateMaxTaskSpecs(count int) []v1alpha1.TaskSpec {
	tasks := make([]v1alpha1.TaskSpec, count)
	for i := 0; i < count; i++ {
		tasks[i] = v1alpha1.TaskSpec{
			Name:     fmt.Sprintf("task-%d", i),
			Replicas: 1,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.14",
						},
					},
				},
			},
		}
	}
	return tasks
}

// MetricsCollector collects and analyzes test metrics
type MetricsCollector struct {
	mutex           sync.Mutex
	jobComparisons  []JobComparisonMetric
	perfComparisons []PerformanceComparison
	burstResults    []BurstLoadResult
}

// JobComparisonMetric represents metrics for a single job validation comparison
type JobComparisonMetric struct {
	ScenarioName     string
	ComparisonResult *ComparisonResult
	Timestamp        time.Time
}

// PerformanceComparison represents performance comparison between webhook and VAP
type PerformanceComparison struct {
	ResourceType    string
	WebhookLatency  LatencyStats
	VAPLatency      LatencyStats
	Timestamp       time.Time
}

// BurstLoadResult represents results from burst load testing
type BurstLoadResult struct {
	Duration         time.Duration
	RequestCount     int
	WebhookErrorRate float64
	VAPErrorRate     float64
	Timestamp        time.Time
}

// LatencyStats contains latency statistics
type LatencyStats struct {
	samples []time.Duration
}

// P95 returns the 95th percentile latency
func (l *LatencyStats) P95() time.Duration {
	return l.percentile(0.95)
}

// P99 returns the 99th percentile latency
func (l *LatencyStats) P99() time.Duration {
	return l.percentile(0.99)
}

// percentile calculates the specified percentile
func (l *LatencyStats) percentile(p float64) time.Duration {
	if len(l.samples) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(l.samples))
	copy(sorted, l.samples)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	index := int(float64(len(sorted)) * p)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		jobComparisons:  []JobComparisonMetric{},
		perfComparisons: []PerformanceComparison{},
		burstResults:    []BurstLoadResult{},
	}
}

// RecordJobComparison records a job validation comparison
func (m *MetricsCollector) RecordJobComparison(scenarioName string, comparison *ComparisonResult) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.jobComparisons = append(m.jobComparisons, JobComparisonMetric{
		ScenarioName:     scenarioName,
		ComparisonResult: comparison,
		Timestamp:        time.Now(),
	})
}

// RecordPerformanceComparison records performance comparison metrics
func (m *MetricsCollector) RecordPerformanceComparison(resourceType string, result interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Implementation would depend on the specific result type
	// For now, placeholder
}

// RecordBurstLoadComparison records burst load test results
func (m *MetricsCollector) RecordBurstLoadComparison(result BurstLoadResult) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.burstResults = append(m.burstResults, result)
}

// GenerateReport generates a comprehensive test report
func (m *MetricsCollector) GenerateReport() string {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	report := "VAP-Webhook Equivalence Test Report\n"
	report += "===================================\n\n"

	// Calculate overall equivalence score
	totalScore := 0.0
	for _, comp := range m.jobComparisons {
		totalScore += comp.ComparisonResult.EquivalenceScore
	}

	if len(m.jobComparisons) > 0 {
		avgScore := totalScore / float64(len(m.jobComparisons))
		report += fmt.Sprintf("Overall Equivalence Score: %.2f%%\n", avgScore*100)
		report += fmt.Sprintf("Total Test Scenarios: %d\n", len(m.jobComparisons))
	}

	// Count failures
	failures := 0
	for _, comp := range m.jobComparisons {
		if !comp.ComparisonResult.Match {
			failures++
		}
	}

	report += fmt.Sprintf("Failed Scenarios: %d\n", failures)
	report += fmt.Sprintf("Success Rate: %.1f%%\n\n", 
		float64(len(m.jobComparisons)-failures)/float64(len(m.jobComparisons))*100)

	// List failed scenarios
	if failures > 0 {
		report += "Failed Scenarios:\n"
		for _, comp := range m.jobComparisons {
			if !comp.ComparisonResult.Match {
				report += fmt.Sprintf("- %s: %v\n", comp.ScenarioName, comp.ComparisonResult.Differences)
			}
		}
	}

	return report
}