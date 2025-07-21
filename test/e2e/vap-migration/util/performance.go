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
	"context"
	"fmt"
	"sync"
	"time"

	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

// PerformanceTest manages performance testing between webhook and VAP
type PerformanceTest struct {
	webhookClient *WebhookTestClient
	vapClient     *VAPTestClient
}

// NewPerformanceTest creates a new performance test instance
func NewPerformanceTest(webhookClient *WebhookTestClient, vapClient *VAPTestClient) *PerformanceTest {
	return &PerformanceTest{
		webhookClient: webhookClient,
		vapClient:     vapClient,
	}
}

// JobValidationPerformanceResult contains results of job validation performance test
type JobValidationPerformanceResult struct {
	WebhookLatency LatencyStats
	VAPLatency     LatencyStats
	WebhookErrors  int
	VAPErrors      int
	TotalRequests  int
}

// RunJobValidationComparison runs a performance comparison test for job validation
func (p *PerformanceTest) RunJobValidationComparison(iterations int) *JobValidationPerformanceResult {
	result := &JobValidationPerformanceResult{
		TotalRequests: iterations,
	}

	// Create test job spec
	testJobSpec := &v1alpha1.JobSpec{
		MinAvailable: 1,
		Queue:        "default",
		Tasks: []v1alpha1.TaskSpec{
			{
				Name:     "test-task",
				Replicas: 1,
			},
		},
	}

	// Test webhook performance
	webhookLatencies := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		webhookResult, err := p.webhookClient.ValidateJob("default", testJobSpec)
		if err != nil {
			result.WebhookErrors++
			continue
		}
		webhookLatencies = append(webhookLatencies, webhookResult.Latency)
	}
	result.WebhookLatency = LatencyStats{samples: webhookLatencies}

	// Test VAP performance
	vapLatencies := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		vapResult, err := p.vapClient.ValidateJob("default", testJobSpec)
		if err != nil {
			result.VAPErrors++
			continue
		}
		vapLatencies = append(vapLatencies, vapResult.Latency)
	}
	result.VAPLatency = LatencyStats{samples: vapLatencies}

	return result
}

// RunBurstLoadTest runs a burst load test comparing webhook and VAP performance
func (p *PerformanceTest) RunBurstLoadTest(requestsPerSecond int, duration time.Duration) BurstLoadResult {
	result := BurstLoadResult{
		Duration: duration,
	}

	// Calculate interval between requests
	interval := time.Second / time.Duration(requestsPerSecond)
	
	// Create test context
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// Counters
	var webhookErrors, vapErrors, totalRequests int
	var wg sync.WaitGroup
	var mutex sync.Mutex

	// Test job spec
	testJobSpec := &v1alpha1.JobSpec{
		MinAvailable: 1,
		Queue:        "default",
		Tasks: []v1alpha1.TaskSpec{
			{
				Name:     "burst-test-task",
				Replicas: 1,
			},
		},
	}

	// Start ticker for request generation
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	requestLoop:
	for {
		select {
		case <-ctx.Done():
			break requestLoop
		case <-ticker.C:
			wg.Add(2)
			
			// Test webhook
			go func() {
				defer wg.Done()
				_, err := p.webhookClient.ValidateJob("default", testJobSpec)
				mutex.Lock()
				totalRequests++
				if err != nil {
					webhookErrors++
				}
				mutex.Unlock()
			}()

			// Test VAP
			go func() {
				defer wg.Done()
				_, err := p.vapClient.ValidateJob("default", testJobSpec)
				mutex.Lock()
				if err != nil {
					vapErrors++
				}
				mutex.Unlock()
			}()
		}
	}

	wg.Wait()

	result.RequestCount = totalRequests
	if totalRequests > 0 {
		result.WebhookErrorRate = float64(webhookErrors) / float64(totalRequests)
		result.VAPErrorRate = float64(vapErrors) / float64(totalRequests)
	}
	result.Timestamp = time.Now()

	return result
}

// LoadTestConfig represents configuration for load testing
type LoadTestConfig struct {
	Duration            time.Duration
	RequestsPerSecond   int
	ResourceTypes       []string
	ValidInvalidRatio   float64 // 0.7 means 70% valid, 30% invalid
}

// RunLoadTest runs a comprehensive load test across multiple resource types
func (p *PerformanceTest) RunLoadTest(config LoadTestConfig) map[string]BurstLoadResult {
	results := make(map[string]BurstLoadResult)

	for _, resourceType := range config.ResourceTypes {
		switch resourceType {
		case "jobs":
			results[resourceType] = p.RunBurstLoadTest(config.RequestsPerSecond, config.Duration)
		// Add cases for other resource types as needed
		default:
			fmt.Printf("Unknown resource type for load testing: %s\n", resourceType)
		}
	}

	return results
}