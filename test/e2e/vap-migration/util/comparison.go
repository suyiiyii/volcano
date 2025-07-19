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
	"math"
	"strings"
	"time"
)

// ComparisonResult contains the result of comparing webhook vs VAP validation
type ComparisonResult struct {
	Match            bool
	Differences      []string
	LatencyDelta     time.Duration
	EquivalenceScore float64
	Recommendations  []string
}

// ComparisonEngine compares webhook vs VAP validation results
type ComparisonEngine struct {
	strictMode bool
	tolerances map[string]interface{}
}

// NewComparisonEngine creates a new comparison engine
func NewComparisonEngine(strictMode bool) *ComparisonEngine {
	return &ComparisonEngine{
		strictMode: strictMode,
		tolerances: map[string]interface{}{
			"latency_tolerance_ms": 50,  // 50ms tolerance
			"message_similarity":   0.8, // 80% message similarity required
		},
	}
}

// CompareJobValidation compares job validation results
func (c *ComparisonEngine) CompareJobValidation(webhookResult, vapResult *ValidationResult) *ComparisonResult {
	result := &ComparisonResult{
		Match:        true,
		Differences:  []string{},
		LatencyDelta: vapResult.Latency - webhookResult.Latency,
	}

	// Compare allowed status
	if webhookResult.Allowed != vapResult.Allowed {
		result.Match = false
		result.Differences = append(result.Differences, 
			fmt.Sprintf("Allowed status mismatch: webhook=%t, vap=%t", 
				webhookResult.Allowed, vapResult.Allowed))
	}

	// Compare error messages if both failed
	if !webhookResult.Allowed && !vapResult.Allowed {
		similarity := c.calculateMessageSimilarity(webhookResult.ErrorMessage, vapResult.ErrorMessage)
		if similarity < c.tolerances["message_similarity"].(float64) {
			result.Match = false
			result.Differences = append(result.Differences,
				fmt.Sprintf("Error message similarity too low: %.2f (threshold: %.2f)", 
					similarity, c.tolerances["message_similarity"].(float64)))
			result.Differences = append(result.Differences,
				fmt.Sprintf("Webhook: %s", webhookResult.ErrorMessage))
			result.Differences = append(result.Differences,
				fmt.Sprintf("VAP: %s", vapResult.ErrorMessage))
		}
	}

	// Check latency tolerance in strict mode
	if c.strictMode {
		latencyToleranceMs := time.Duration(c.tolerances["latency_tolerance_ms"].(int)) * time.Millisecond
		if math.Abs(float64(result.LatencyDelta)) > float64(latencyToleranceMs) {
			result.Differences = append(result.Differences,
				fmt.Sprintf("Latency difference too high: %v (threshold: %v)", 
					result.LatencyDelta, latencyToleranceMs))
		}
	}

	// Calculate equivalence score
	result.EquivalenceScore = c.calculateEquivalenceScore(webhookResult, vapResult, result.Differences)

	// Generate recommendations
	result.Recommendations = c.generateRecommendations(result.Differences)

	return result
}

// calculateMessageSimilarity calculates similarity between two error messages
func (c *ComparisonEngine) calculateMessageSimilarity(msg1, msg2 string) float64 {
	if msg1 == msg2 {
		return 1.0
	}

	// Simple word-based similarity calculation
	words1 := strings.Fields(strings.ToLower(msg1))
	words2 := strings.Fields(strings.ToLower(msg2))

	if len(words1) == 0 && len(words2) == 0 {
		return 1.0
	}

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Count common words
	wordCount := make(map[string]int)
	for _, word := range words1 {
		wordCount[word]++
	}

	common := 0
	for _, word := range words2 {
		if wordCount[word] > 0 {
			common++
			wordCount[word]--
		}
	}

	// Jaccard similarity approximation
	union := len(words1) + len(words2) - common
	if union == 0 {
		return 1.0
	}

	return float64(common) / float64(union)
}

// calculateEquivalenceScore calculates overall equivalence score (0-1)
func (c *ComparisonEngine) calculateEquivalenceScore(webhookResult, vapResult *ValidationResult, differences []string) float64 {
	score := 1.0

	// Major deduction for allowed status mismatch
	if webhookResult.Allowed != vapResult.Allowed {
		score -= 0.5
	}

	// Minor deductions for other differences
	for _, diff := range differences {
		if strings.Contains(diff, "Latency") {
			score -= 0.1
		} else if strings.Contains(diff, "message") {
			score -= 0.2
		} else {
			score -= 0.15
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

// generateRecommendations generates recommendations based on differences
func (c *ComparisonEngine) generateRecommendations(differences []string) []string {
	recommendations := []string{}

	for _, diff := range differences {
		if strings.Contains(diff, "Allowed status mismatch") {
			recommendations = append(recommendations,
				"Critical: Review VAP expressions to ensure they match webhook validation logic")
		} else if strings.Contains(diff, "message similarity") {
			recommendations = append(recommendations,
				"Review VAP message expressions to match webhook error messages more closely")
		} else if strings.Contains(diff, "Latency") {
			recommendations = append(recommendations,
				"Performance: Consider optimizing VAP expressions for better performance")
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Validation results are equivalent - good to proceed!")
	}

	return recommendations
}