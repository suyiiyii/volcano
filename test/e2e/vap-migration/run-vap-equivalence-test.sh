#!/bin/bash

# Volcano VAP Migration Equivalence Test Runner
# This script provides comprehensive testing for ValidatingAdmissionPolicy equivalence with webhooks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VOLCANO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
TEST_NAMESPACE="volcano-vap-test"
POLICIES_DIR="$VOLCANO_ROOT/config/validating-admission-policies"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

success() {
    echo -e "${GREEN}[SUCCESS] $1${NC}"
}

warning() {
    echo -e "${YELLOW}[WARNING] $1${NC}"
}

error() {
    echo -e "${RED}[ERROR] $1${NC}"
}

info() {
    echo -e "${PURPLE}[INFO] $1${NC}"
}

# Function to check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        error "kubectl not found. Please install kubectl."
        exit 1
    fi
    
    # Check if cluster is available
    if ! kubectl cluster-info &> /dev/null; then
        error "No Kubernetes cluster available. Please ensure cluster is running."
        exit 1
    fi
    
    # Check Kubernetes version for VAP support
    K8S_VERSION=$(kubectl version --output=json | jq -r '.serverVersion.major + "." + .serverVersion.minor')
    log "Kubernetes version: $K8S_VERSION"
    
    # Check if ValidatingAdmissionPolicy is supported
    if kubectl get validatingadmissionpolicies &> /dev/null; then
        success "ValidatingAdmissionPolicy API is available"
    else
        error "ValidatingAdmissionPolicy API not available. Kubernetes v1.32+ required."
        exit 1
    fi
    
    success "Prerequisites check passed"
}

# Function to setup test environment
setup_test_environment() {
    log "Setting up test environment..."
    
    # Create test namespace
    kubectl create namespace "$TEST_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
    
    # Create volcano-system namespace if needed
    kubectl create namespace volcano-system --dry-run=client -o yaml | kubectl apply -f -
    
    success "Test environment ready"
}

# Function to deploy ValidatingAdmissionPolicies
deploy_vap_policies() {
    log "Deploying ValidatingAdmissionPolicies..."
    
    for policy_file in "$POLICIES_DIR"/*.yaml; do
        if [[ -f "$policy_file" ]]; then
            policy_name=$(basename "$policy_file")
            log "Applying $policy_name..."
            kubectl apply -f "$policy_file"
        fi
    done
    
    # Wait for policies to be ready
    sleep 10
    
    # Verify policies are deployed
    VAP_COUNT=$(kubectl get validatingadmissionpolicies --no-headers | wc -l)
    if [[ $VAP_COUNT -gt 0 ]]; then
        success "Deployed $VAP_COUNT ValidatingAdmissionPolicies"
    else
        error "No ValidatingAdmissionPolicies found after deployment"
        exit 1
    fi
}

# Function to run equivalence tests
run_equivalence_tests() {
    log "Running VAP-Webhook equivalence tests..."
    
    # Change to test directory
    cd "$SCRIPT_DIR"
    
    # Run the Go tests using Ginkgo
    if command -v ginkgo &> /dev/null; then
        log "Running tests with Ginkgo..."
        KUBECONFIG=${KUBECONFIG} ginkgo -v -r --slow-spec-threshold='60s' --progress .
    else
        log "Running tests with go test..."
        KUBECONFIG=${KUBECONFIG} go test -v -timeout 30m ./...
    fi
    
    TEST_EXIT_CODE=$?
    
    if [[ $TEST_EXIT_CODE -eq 0 ]]; then
        success "Equivalence tests PASSED"
    else
        error "Equivalence tests FAILED"
    fi
    
    return $TEST_EXIT_CODE
}

# Function to generate test report
generate_test_report() {
    log "Generating comprehensive test report..."
    
    REPORT_FILE="$VOLCANO_ROOT/vap-migration-test-report.md"
    
    cat > "$REPORT_FILE" << EOF
# Volcano VAP Migration Equivalence Test Report

Generated on: $(date)
Kubernetes Version: $(kubectl version --short)
Test Environment: $TEST_NAMESPACE

## Test Summary

EOF
    
    # Add test results summary
    if [[ $1 -eq 0 ]]; then
        echo "✅ **Overall Result: PASSED**" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        echo "All ValidatingAdmissionPolicy validations are functionally equivalent to the original webhooks." >> "$REPORT_FILE"
    else
        echo "❌ **Overall Result: FAILED**" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        echo "Some ValidatingAdmissionPolicy validations differ from webhook behavior. Review detailed results above." >> "$REPORT_FILE"
    fi
    
    cat >> "$REPORT_FILE" << EOF

## Deployed ValidatingAdmissionPolicies

EOF
    
    kubectl get validatingadmissionpolicies -o custom-columns="NAME:.metadata.name,MATCH RESOURCES:.spec.matchConstraints.resourceRules[*].resources" >> "$REPORT_FILE"
    
    cat >> "$REPORT_FILE" << EOF

## Test Environment Details

### Namespaces
EOF
    
    kubectl get namespaces | grep -E "(volcano|$TEST_NAMESPACE)" >> "$REPORT_FILE" || true
    
    cat >> "$REPORT_FILE" << EOF

### Cluster Info
EOF
    
    kubectl cluster-info >> "$REPORT_FILE"
    
    success "Test report generated: $REPORT_FILE"
}

# Function to cleanup test environment
cleanup_test_environment() {
    log "Cleaning up test environment..."
    
    # Remove test namespace
    kubectl delete namespace "$TEST_NAMESPACE" --ignore-not-found=true
    
    # Optionally remove VAP policies (ask user)
    if [[ "${CLEANUP_POLICIES:-0}" == "1" ]]; then
        log "Removing ValidatingAdmissionPolicies..."
        for policy_file in "$POLICIES_DIR"/*.yaml; do
            if [[ -f "$policy_file" ]]; then
                kubectl delete -f "$policy_file" --ignore-not-found=true
            fi
        done
    else
        info "ValidatingAdmissionPolicies left in place. Set CLEANUP_POLICIES=1 to remove them."
    fi
    
    success "Cleanup completed"
}

# Function to display usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --setup-only     Only setup test environment, don't run tests"
    echo "  --test-only      Only run tests (assumes environment is setup)"
    echo "  --cleanup-only   Only cleanup test environment"
    echo "  --no-cleanup     Don't cleanup after tests"
    echo "  --help           Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  CLEANUP_POLICIES=1   Remove VAP policies during cleanup"
    echo "  TEST_NAMESPACE       Test namespace name (default: volcano-vap-test)"
}

# Main execution
main() {
    local setup_only=false
    local test_only=false
    local cleanup_only=false
    local no_cleanup=false
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --setup-only)
                setup_only=true
                shift
                ;;
            --test-only)
                test_only=true
                shift
                ;;
            --cleanup-only)
                cleanup_only=true
                shift
                ;;
            --no-cleanup)
                no_cleanup=true
                shift
                ;;
            --help)
                usage
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
    
    info "=== Volcano VAP Migration Equivalence Testing ==="
    info "Test Namespace: $TEST_NAMESPACE"
    info "Policies Directory: $POLICIES_DIR"
    
    local test_exit_code=0
    
    # Cleanup only mode
    if [[ "$cleanup_only" == true ]]; then
        cleanup_test_environment
        exit 0
    fi
    
    # Check prerequisites (unless test-only mode)
    if [[ "$test_only" != true ]]; then
        check_prerequisites
    fi
    
    # Setup environment (unless test-only mode)
    if [[ "$test_only" != true ]]; then
        setup_test_environment
        deploy_vap_policies
    fi
    
    # Exit if setup-only mode
    if [[ "$setup_only" == true ]]; then
        success "Environment setup completed"
        exit 0
    fi
    
    # Run tests
    run_equivalence_tests || test_exit_code=$?
    
    # Generate report
    generate_test_report $test_exit_code
    
    # Cleanup (unless no-cleanup specified)
    if [[ "$no_cleanup" != true ]]; then
        cleanup_test_environment
    fi
    
    if [[ $test_exit_code -eq 0 ]]; then
        success "=== VAP Migration Equivalence Testing COMPLETED SUCCESSFULLY ==="
    else
        error "=== VAP Migration Equivalence Testing FAILED ==="
    fi
    
    exit $test_exit_code
}

# Run main function with all arguments
main "$@"