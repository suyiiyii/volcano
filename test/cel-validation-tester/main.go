package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// TestCase represents a validation test case
type TestCase struct {
	Name        string                 `json:"name"`
	Object      map[string]interface{} `json:"object"`
	OldObject   map[string]interface{} `json:"oldObject,omitempty"`
	Operation   string                 `json:"operation"`
	ExpectValid bool                   `json:"expectValid"`
	Description string                 `json:"description"`
}

// ValidationPolicy represents a CEL-based validation policy
type ValidationPolicy struct {
	Name        string `json:"name"`
	Expression  string `json:"expression"`
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
}

// PolicyValidator validates objects using CEL expressions
type PolicyValidator struct {
	env *cel.Env
}

// NewPolicyValidator creates a new policy validator with Kubernetes-aware CEL environment
func NewPolicyValidator() (*PolicyValidator, error) {
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("object", decls.NewMapType(decls.String, decls.Dyn)),
			decls.NewVar("oldObject", decls.NewMapType(decls.String, decls.Dyn)),
			decls.NewVar("request", decls.NewMapType(decls.String, decls.Dyn)),
			decls.NewVar("variables", decls.NewMapType(decls.String, decls.Dyn)),
		),
		cel.OptionalTypes(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %v", err)
	}

	return &PolicyValidator{env: env}, nil
}

// ValidateExpression validates a single CEL expression against an object
func (pv *PolicyValidator) ValidateExpression(expression string, object, oldObject map[string]interface{}, operation string) (bool, error) {
	ast, issues := pv.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("expression compilation failed: %v", issues.Err())
	}

	prg, err := pv.env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("program creation failed: %v", err)
	}

	// Prepare evaluation context
	vars := map[string]interface{}{
		"object": object,
		"request": map[string]interface{}{
			"operation": operation,
		},
	}

	if oldObject != nil {
		vars["oldObject"] = oldObject
	}

	// Calculate variables (simplified - in real implementation, these would be pre-computed)
	vars["variables"] = pv.calculateVariables(object, oldObject)

	out, _, err := prg.Eval(vars)
	if err != nil {
		return false, fmt.Errorf("expression evaluation failed: %v", err)
	}

	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("expression did not evaluate to boolean, got: %T", out.Value())
	}

	return result, nil
}

// calculateVariables calculates common variables used in validation policies
func (pv *PolicyValidator) calculateVariables(object, oldObject map[string]interface{}) map[string]interface{} {
	variables := make(map[string]interface{})

	// Extract spec if it exists
	if spec, ok := object["spec"].(map[string]interface{}); ok {
		// Calculate totalReplicas for Jobs
		if tasks, ok := spec["tasks"].([]interface{}); ok {
			totalReplicas := int32(0)
			taskNames := make([]string, 0, len(tasks))
			
			for _, task := range tasks {
				if taskMap, ok := task.(map[string]interface{}); ok {
					if replicas, ok := taskMap["replicas"].(int); ok {
						totalReplicas += int32(replicas)
					}
					if name, ok := taskMap["name"].(string); ok {
						taskNames = append(taskNames, name)
					}
				}
			}
			variables["totalReplicas"] = totalReplicas
			variables["taskNames"] = taskNames
		}

		// Calculate flowNames for JobFlows
		if flows, ok := spec["flows"].([]interface{}); ok {
			flowNames := make([]string, 0, len(flows))
			for _, flow := range flows {
				if flowMap, ok := flow.(map[string]interface{}); ok {
					if name, ok := flowMap["name"].(string); ok {
						flowNames = append(flowNames, name)
					}
				}
			}
			variables["flowNames"] = flowNames
			variables["hasFlows"] = len(flows) > 0
		}

		// Other variables
		if queue, ok := spec["queue"].(string); ok {
			variables["hasQueue"] = queue != ""
		}
		
		if members, ok := spec["members"].([]interface{}); ok {
			variables["hasMembers"] = len(members) > 0
		}
	}

	return variables
}

// LoadTestCases loads test cases from a JSON file
func LoadTestCases(filename string) ([]TestCase, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read test cases file: %v", err)
	}

	var testCases []TestCase
	if err := json.Unmarshal(data, &testCases); err != nil {
		return nil, fmt.Errorf("failed to unmarshal test cases: %v", err)
	}

	return testCases, nil
}

// LoadValidationPolicies loads validation policies from a JSON file
func LoadValidationPolicies(filename string) ([]ValidationPolicy, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read policies file: %v", err)
	}

	var policies []ValidationPolicy
	if err := json.Unmarshal(data, &policies); err != nil {
		return nil, fmt.Errorf("failed to unmarshal policies: %v", err)
	}

	return policies, nil
}

// RunValidationTests runs all validation tests against the policies
func RunValidationTests(validator *PolicyValidator, policies []ValidationPolicy, testCases []TestCase) {
	fmt.Printf("Running %d test cases against %d policies...\n\n", len(testCases), len(policies))

	totalTests := 0
	passedTests := 0
	failedTests := 0

	for _, testCase := range testCases {
		fmt.Printf("Test Case: %s\n", testCase.Name)
		fmt.Printf("Description: %s\n", testCase.Description)
		fmt.Printf("Expected Valid: %t\n", testCase.ExpectValid)

		allPoliciesPassed := true
		policyResults := make([]bool, 0, len(policies))

		// Test against each policy
		for _, policy := range policies {
			valid, err := validator.ValidateExpression(
				policy.Expression,
				testCase.Object,
				testCase.OldObject,
				testCase.Operation,
			)
			if err != nil {
				fmt.Printf("  ❌ Policy '%s' - Error: %v\n", policy.Name, err)
				allPoliciesPassed = false
				continue
			}

			policyResults = append(policyResults, valid)
			if valid {
				fmt.Printf("  ✅ Policy '%s' - PASS\n", policy.Name)
			} else {
				fmt.Printf("  ❌ Policy '%s' - FAIL: %s\n", policy.Name, policy.Message)
				allPoliciesPassed = false
			}
		}

		// Overall result for this test case
		overallValid := allPoliciesPassed
		testPassed := (overallValid == testCase.ExpectValid)
		
		if testPassed {
			fmt.Printf("Result: ✅ PASS (Overall Valid: %t, Expected: %t)\n", overallValid, testCase.ExpectValid)
			passedTests++
		} else {
			fmt.Printf("Result: ❌ FAIL (Overall Valid: %t, Expected: %t)\n", overallValid, testCase.ExpectValid)
			failedTests++
		}

		totalTests++
		fmt.Println()
	}

	fmt.Printf("Test Summary:\n")
	fmt.Printf("Total Tests: %d\n", totalTests)
	fmt.Printf("Passed: %d\n", passedTests)
	fmt.Printf("Failed: %d\n", failedTests)
	fmt.Printf("Success Rate: %.2f%%\n", float64(passedTests)/float64(totalTests)*100)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %s <policies.json> <testcases.json>\n", os.Args[0])
		os.Exit(1)
	}

	policiesFile := os.Args[1]
	testCasesFile := os.Args[2]

	// Create validator
	validator, err := NewPolicyValidator()
	if err != nil {
		log.Fatalf("Failed to create validator: %v", err)
	}

	// Load policies
	policies, err := LoadValidationPolicies(policiesFile)
	if err != nil {
		log.Fatalf("Failed to load policies: %v", err)
	}

	// Load test cases
	testCases, err := LoadTestCases(testCasesFile)
	if err != nil {
		log.Fatalf("Failed to load test cases: %v", err)
	}

	// Run tests
	RunValidationTests(validator, policies, testCases)
}