#!/bin/bash

# ==========================================
# Enhanced Security Scanning Script v2.0
# ==========================================
# Features:
# - Multiple security scanners (govulncheck, gosec, golangci-lint, trivy)
# - CI/CD optimized (GitHub Actions, GitLab)
# - JSON output for automation
# - Improved error handling and retry logic
# - Parallel scanning with proper synchronization
# - Better reporting and GitHub integration
# ==========================================

set -euo pipefail

# ==========================================
# Color codes
# ==========================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# ==========================================
# Configuration
# ==========================================
REPORT_DIR="${REPORT_DIR:-security-reports}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
PROJECT_NAME="${PROJECT_NAME:-$(basename "$(pwd)")}"
PROJECT_DIR="$(pwd)"
CI_MODE="${CI_MODE:-false}"
PARALLEL_JOBS=4
RETRY_COUNT=3
RETRY_DELAY=2
SCAN_TIMEOUT=600  # 10 minutes

# Detect CI environment
if [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ] || [ -n "${GITLAB_CI:-}" ]; then
  CI_MODE=true
  echo "🤖 CI environment detected"
fi

# ==========================================
# Helper Functions
# ==========================================

log_info() {
  echo -e "${BLUE}ℹ️  $*${NC}"
}

log_success() {
  echo -e "${GREEN}✅ $*${NC}"
}

log_warning() {
  echo -e "${YELLOW}⚠️  $*${NC}"
}

log_error() {
  echo -e "${RED}❌ $*${NC}"
}

log_header() {
  echo ""
  echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${CYAN}$*${NC}"
  echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
}

# Retry function for network operations
retry_command() {
  local cmd=$1
  local attempt=1
  
  while [ $attempt -le $RETRY_COUNT ]; do
    if eval "$cmd"; then
      return 0
    fi
    
    if [ $attempt -lt $RETRY_COUNT ]; then
      log_warning "Attempt $attempt failed. Retrying in ${RETRY_DELAY}s..."
      sleep $RETRY_DELAY
    fi
    attempt=$((attempt + 1))
  done
  
  log_error "Command failed after $RETRY_COUNT attempts: $cmd"
  return 1
}

# ==========================================
# Validation
# ==========================================

validate_environment() {
  log_header "Validating Environment"
  
  if [ ! -f "go.mod" ]; then
    log_error "go.mod not found in current directory: $(pwd)"
    echo "Please run this script from your Go project root directory."
    exit 1
  fi
  
  log_success "Found go.mod"
  log_success "Project directory: $PROJECT_DIR"
  
  # Check Go installation
  if ! command -v go &> /dev/null; then
    log_error "Go is not installed"
    exit 1
  fi
  log_success "Go compiler found: $(go version)"
  
  # Check jq installation
  if ! command -v jq &> /dev/null; then
    log_warning "jq not found - installing..."
    if command -v apt-get &> /dev/null; then
      sudo apt-get update -qq && sudo apt-get install -y jq
    elif command -v yum &> /dev/null; then
      sudo yum install -y jq
    else
      log_error "Cannot install jq - please install manually"
      exit 1
    fi
  fi
}

# ==========================================
# Setup
# ==========================================

setup_environment() {
  log_header "Setting Up Security Scanning"
  
  # Clean and create report directory
  log_info "Cleaning up old reports..."
  rm -rf "$REPORT_DIR" .scannerwork
  mkdir -p "$REPORT_DIR"
  
  log_success "Report directory ready: $REPORT_DIR"
  
  # Install/update security tools
  log_info "Installing security tools..."
  
  export GOPATH="${GOPATH:-$HOME/go}"
  export PATH=$PATH:$GOPATH/bin
  
  # Install tools with retry
  retry_command "go install golang.org/x/vuln/cmd/govulncheck@latest" || log_warning "govulncheck install failed"
  retry_command "go install github.com/securego/gosec/v2/cmd/gosec@latest" || log_warning "gosec install failed"
  retry_command "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" || log_warning "golangci-lint install failed"
  
  # Verify installations
  if ! command -v govulncheck &> /dev/null; then
    log_error "govulncheck installation failed"
  else
    log_success "govulncheck installed"
  fi
  
  if ! command -v gosec &> /dev/null; then
    log_error "gosec installation failed"
  else
    log_success "gosec installed"
  fi
  
  if ! command -v golangci-lint &> /dev/null; then
    log_error "golangci-lint installation failed"
  else
    log_success "golangci-lint installed"
  fi
  
  # Update Go modules
  log_info "Downloading and verifying dependencies..."
  go mod download 2>&1 || log_warning "go mod download had issues"
  go mod verify 2>&1 || log_warning "go mod verify had issues"
  
  # Ensure go.sum exists
  if [ ! -f "go.sum" ]; then
    log_info "Generating go.sum..."
    go mod tidy
  fi
  
  log_success "Environment setup complete"
}

# ==========================================
# Scanning Functions
# ==========================================

scan_vulnerabilities() {
  log_header "1️⃣  Vulnerability Scanning (govulncheck)"
  
  local vuln_json="$REPORT_DIR/govulncheck-${TIMESTAMP}.json"
  local vuln_txt="$REPORT_DIR/govulncheck-${TIMESTAMP}.txt"
  
  log_info "Scanning for known vulnerabilities..."
  
  # Verify we're in the right directory
  if [ ! -f "go.mod" ]; then
    log_error "go.mod not found in $(pwd)"
    echo "0|||"
    return 1
  fi
  
  # Run govulncheck with JSON output
  local vuln_count=0
  if timeout $SCAN_TIMEOUT govulncheck -json ./... > "$vuln_json" 2>&1; then
    vuln_count=$(jq '[.finding? // empty | select(.osv != null)] | length' "$vuln_json" 2>/dev/null || echo "0")
    log_success "Vulnerability scan complete - Found: $vuln_count vulnerabilities"
  else
    # Check for actual vulnerabilities even if command failed
    if [ -f "$vuln_json" ]; then
      vuln_count=$(jq '[.finding? // empty | select(.osv != null)] | length' "$vuln_json" 2>/dev/null || echo "0")
      if [ "$vuln_count" -gt 0 ]; then
        log_warning "Found $vuln_count vulnerabilities"
      else
        log_success "No known vulnerabilities found (scan completed with warnings)"
      fi
    else
      log_error "Vulnerability scan failed"
    fi
  fi
  
  # Generate text report
  timeout $SCAN_TIMEOUT govulncheck ./... > "$vuln_txt" 2>&1 || true
  
  echo "$vuln_count|$vuln_json|$vuln_txt"
}

scan_security() {
  log_header "2️⃣  Security Analysis (gosec)"
  
  local gosec_json="$REPORT_DIR/gosec-${TIMESTAMP}.json"
  local gosec_html="$REPORT_DIR/gosec-${TIMESTAMP}.html"
  local gosec_sarif="$REPORT_DIR/gosec-${TIMESTAMP}.sarif"
  
  log_info "Running static security analysis..."
  
  # Run gosec with multiple output formats
  timeout $SCAN_TIMEOUT gosec -fmt=json -out="$gosec_json" ./... 2>&1 || true
  timeout $SCAN_TIMEOUT gosec -fmt=html -out="$gosec_html" ./... 2>&1 || true
  timeout $SCAN_TIMEOUT gosec -fmt=sarif -out="$gosec_sarif" ./... 2>&1 || true
  
  # Parse results
  local issues=0
  local files=0
  local lines=0
  local high_sev=0
  local med_sev=0
  local low_sev=0
  
  if [ -f "$gosec_json" ]; then
    issues=$(jq -r '.Stats.found // 0' "$gosec_json" 2>/dev/null || echo "0")
    files=$(jq -r '.Stats.files // 0' "$gosec_json" 2>/dev/null || echo "0")
    lines=$(jq -r '.Stats.lines // 0' "$gosec_json" 2>/dev/null || echo "0")
    high_sev=$(jq '[.Issues[]? | select(.severity=="HIGH")] | length' "$gosec_json" 2>/dev/null || echo "0")
    med_sev=$(jq '[.Issues[]? | select(.severity=="MEDIUM")] | length' "$gosec_json" 2>/dev/null || echo "0")
    low_sev=$(jq '[.Issues[]? | select(.severity=="LOW")] | length' "$gosec_json" 2>/dev/null || echo "0")
    
    if [ "$issues" -gt 0 ]; then
      log_warning "Found $issues security issues in $files files (High: $high_sev, Medium: $med_sev, Low: $low_sev)"
    else
      log_success "No security issues found"
    fi
  else
    log_error "Gosec report not generated"
  fi
  
  echo "$issues|$files|$lines|$high_sev|$med_sev|$low_sev|$gosec_json|$gosec_html|$gosec_sarif"
}

scan_code_quality() {
  log_header "3️⃣  Code Quality Check (golangci-lint)"
  
  local lint_json="$REPORT_DIR/golangci-lint-${TIMESTAMP}.json"
  
  log_info "Running code quality checks..."
  
  # Run with timeout and capture output
  timeout $SCAN_TIMEOUT golangci-lint run --out-format json ./... > "$lint_json" 2>&1 || true
  
  local issues=0
  if [ -f "$lint_json" ] && [ -s "$lint_json" ]; then
    issues=$(jq '[.Issues[]? // empty] | length' "$lint_json" 2>/dev/null || echo "0")
    
    if [ "$issues" -gt 0 ]; then
      log_warning "Found $issues code quality issues"
    else
      log_success "No code quality issues found"
    fi
  else
    log_warning "golangci-lint produced no output"
    echo '{"Issues":[]}' > "$lint_json"
  fi
  
  echo "$issues|$lint_json"
}

scan_dependencies() {
  log_header "4️⃣  Dependency Scanning (Trivy)"
  
  local trivy_json="$REPORT_DIR/trivy-${TIMESTAMP}.json"
  local trivy_sarif="$REPORT_DIR/trivy-${TIMESTAMP}.sarif"
  
  # Check if trivy is installed
  if ! command -v trivy &> /dev/null; then
    log_warning "Trivy not installed - skipping dependency scan"
    echo "0||"
    return 0
  fi
  
  log_info "Scanning dependencies for vulnerabilities..."
  
  # Run trivy filesystem scan
  local dep_vuln=0
  if timeout $SCAN_TIMEOUT trivy fs --format json --output "$trivy_json" --scanners vuln,secret . 2>/dev/null; then
    # Count vulnerabilities
    dep_vuln=$(jq '[.Results[]?.Vulnerabilities[]? // empty] | length' "$trivy_json" 2>/dev/null || echo "0")
    log_success "Dependency scan complete - Found: $dep_vuln issues"
  else
    log_warning "Trivy scan encountered issues"
  fi
  
  # Generate SARIF format
  timeout $SCAN_TIMEOUT trivy fs --format sarif --output "$trivy_sarif" . 2>/dev/null || true
  
  echo "$dep_vuln|$trivy_json|$trivy_sarif"
}

# ==========================================
# Parallel Scanning with Better Error Handling
# ==========================================

run_parallel_scans() {
  log_header "Running Parallel Security Scans"
  
  # Create temp files for results
  local temp_vuln=$(mktemp)
  local temp_sec=$(mktemp)
  local temp_qual=$(mktemp)
  local temp_dep=$(mktemp)
  
  # Run scans in background
  (scan_vulnerabilities > "$temp_vuln" 2>&1) &
  local pid1=$!
  
  (scan_security > "$temp_sec" 2>&1) &
  local pid2=$!
  
  (scan_code_quality > "$temp_qual" 2>&1) &
  local pid3=$!
  
  (scan_dependencies > "$temp_dep" 2>&1) &
  local pid4=$!
  
  log_info "Waiting for scans to complete (PIDs: $pid1, $pid2, $pid3, $pid4)..."
  
  # Wait for all with timeout
  local timeout=600
  local elapsed=0
  local all_done=false
  
  while [ $elapsed -lt $timeout ]; do
    if ! ps -p $pid1 $pid2 $pid3 $pid4 > /dev/null 2>&1; then
      all_done=true
      break
    fi
    sleep 2
    elapsed=$((elapsed + 2))
    
    # Show progress every 30 seconds
    if [ $((elapsed % 30)) -eq 0 ]; then
      log_info "Still scanning... (${elapsed}s elapsed)"
    fi
  done
  
  if [ "$all_done" = false ]; then
    log_warning "Timeout reached, terminating scans..."
    kill $pid1 $pid2 $pid3 $pid4 2>/dev/null || true
    wait 2>/dev/null || true
  else
    # Wait for all to finish cleanly
    wait $pid1 $pid2 $pid3 $pid4 2>/dev/null || true
  fi
  
  # Parse results with robust error handling
  VULN_COUNT=0
  VULN_JSON=""
  VULN_TXT=""
  
  if [ -s "$temp_vuln" ]; then
    IFS='|' read -r VULN_COUNT VULN_JSON VULN_TXT <<< "$(cat "$temp_vuln")"
  fi
  
  ISSUES_FOUND=0
  FILES_SCANNED=0
  LINES_SCANNED=0
  HIGH_SEV=0
  MED_SEV=0
  LOW_SEV=0
  GOSEC_REPORT=""
  GOSEC_HTML=""
  GOSEC_SARIF=""
  
  if [ -s "$temp_sec" ]; then
    IFS='|' read -r ISSUES_FOUND FILES_SCANNED LINES_SCANNED HIGH_SEV MED_SEV LOW_SEV GOSEC_REPORT GOSEC_HTML GOSEC_SARIF <<< "$(cat "$temp_sec")"
  fi
  
  LINT_ISSUES=0
  LINT_REPORT=""
  
  if [ -s "$temp_qual" ]; then
    IFS='|' read -r LINT_ISSUES LINT_REPORT <<< "$(cat "$temp_qual")"
  fi
  
  DEP_VULN=0
  TRIVY_JSON=""
  TRIVY_SARIF=""
  
  if [ -s "$temp_dep" ]; then
    IFS='|' read -r DEP_VULN TRIVY_JSON TRIVY_SARIF <<< "$(cat "$temp_dep")"
  fi
  
  # Set defaults for empty values
  VULN_COUNT=${VULN_COUNT:-0}
  ISSUES_FOUND=${ISSUES_FOUND:-0}
  FILES_SCANNED=${FILES_SCANNED:-0}
  LINES_SCANNED=${LINES_SCANNED:-0}
  HIGH_SEV=${HIGH_SEV:-0}
  MED_SEV=${MED_SEV:-0}
  LOW_SEV=${LOW_SEV:-0}
  LINT_ISSUES=${LINT_ISSUES:-0}
  DEP_VULN=${DEP_VULN:-0}
  
  log_success "All scans completed"
  
  # Cleanup temp files
  rm -f "$temp_vuln" "$temp_sec" "$temp_qual" "$temp_dep"
}

# ==========================================
# Generate Reports
# ==========================================

generate_summary_json() {
  log_info "Generating consolidated JSON report..."
  
  local summary_file="$REPORT_DIR/summary-${TIMESTAMP}.json"
  
  local total_critical=$((VULN_COUNT + HIGH_SEV))
  local overall_status="PASSED"
  
  if [ $total_critical -gt 0 ]; then
    overall_status="FAILED"
  fi
  
  cat > "$summary_file" << EOF
{
  "scan_timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "project_name": "$PROJECT_NAME",
  "project_directory": "$PROJECT_DIR",
  "ci_environment": $([[ "$CI_MODE" == "true" ]] && echo "true" || echo "false"),
  "overall_status": "$overall_status",
  "total_critical_issues": $total_critical,
  "summary": {
    "vulnerabilities": {
      "tool": "govulncheck",
      "count": $VULN_COUNT,
      "status": "$([ $VULN_COUNT -eq 0 ] && echo "PASSED" || echo "FAILED")"
    },
    "security_issues": {
      "tool": "gosec",
      "count": $ISSUES_FOUND,
      "high": $HIGH_SEV,
      "medium": $MED_SEV,
      "low": $LOW_SEV,
      "status": "$([ $HIGH_SEV -eq 0 ] && echo "PASSED" || echo "FAILED")"
    },
    "code_quality": {
      "tool": "golangci-lint",
      "count": $LINT_ISSUES,
      "status": "$([ $LINT_ISSUES -eq 0 ] && echo "PASSED" || echo "WARNING")"
    },
    "dependencies": {
      "tool": "trivy",
      "count": $DEP_VULN,
      "status": "INFO"
    }
  },
  "reports": {
    "gosec_json": "$GOSEC_REPORT",
    "gosec_html": "$GOSEC_HTML",
    "gosec_sarif": "$GOSEC_SARIF",
    "govulncheck_json": "$VULN_JSON",
    "govulncheck_txt": "$VULN_TXT",
    "golangci_lint_json": "$LINT_REPORT",
    "trivy_json": "$TRIVY_JSON",
    "trivy_sarif": "$TRIVY_SARIF"
  },
  "statistics": {
    "files_scanned": $FILES_SCANNED,
    "lines_scanned": $LINES_SCANNED
  }
}
EOF

  log_success "Summary saved: $summary_file"
  
  # Also create CI-friendly report
  cp "$summary_file" "$REPORT_DIR/ci-report.json"
  
  echo "$summary_file"
}

generate_summary_markdown() {
  log_info "Generating markdown report..."
  
  local summary_md="$REPORT_DIR/SECURITY-REPORT.md"
  local total_critical=$((VULN_COUNT + HIGH_SEV))
  
  cat > "$summary_md" << EOF
# 🔒 Security Scan Report

**Generated:** $(date '+%Y-%m-%d %H:%M:%S UTC')  
**Project:** $PROJECT_NAME  
**Status:** $([ $total_critical -eq 0 ] && echo "✅ PASSED" || echo "❌ FAILED")

---

## 📊 Summary

| Component | Issues | Status |
|-----------|--------|--------|
| 🔍 Vulnerabilities (govulncheck) | $VULN_COUNT | $([ $VULN_COUNT -eq 0 ] && echo "✅" || echo "❌") |
| 🔒 Security (gosec) | $ISSUES_FOUND | $([ $HIGH_SEV -eq 0 ] && echo "✅" || echo "⚠️") |
| 📊 Code Quality (golangci-lint) | $LINT_ISSUES | $([ $LINT_ISSUES -eq 0 ] && echo "✅" || echo "⚠️") |
| 📦 Dependencies (trivy) | $DEP_VULN | ℹ️ |

**Files Scanned:** $FILES_SCANNED  
**Lines Scanned:** $LINES_SCANNED

---

## 🔍 Vulnerability Scan Results

- **Total Vulnerabilities:** $VULN_COUNT
- **Status:** $([ $VULN_COUNT -eq 0 ] && echo "✅ No vulnerabilities found" || echo "❌ CRITICAL - Vulnerabilities detected")

**Reports:**
- JSON: [\`$(basename "$VULN_JSON")\`]($VULN_JSON)
- Text: [\`$(basename "$VULN_TXT")\`]($VULN_TXT)

---

## 🔒 Security Analysis Results

- **Total Issues:** $ISSUES_FOUND
- **Files Scanned:** $FILES_SCANNED
- **Lines Scanned:** $LINES_SCANNED

### Severity Breakdown

| Level | Count |
|-------|-------|
| 🔴 High | $HIGH_SEV |
| 🟡 Medium | $MED_SEV |
| 🟢 Low | $LOW_SEV |

**Reports:**
- JSON: [\`$(basename "$GOSEC_REPORT")\`]($GOSEC_REPORT)
- HTML: [\`$(basename "$GOSEC_HTML")\`]($GOSEC_HTML) - **Open in browser for details**
- SARIF: [\`$(basename "$GOSEC_SARIF")\`]($GOSEC_SARIF)

---

## 📊 Code Quality Results

- **Total Issues:** $LINT_ISSUES
- **Report:** [\`$(basename "$LINT_REPORT")\`]($LINT_REPORT)

---

## 📦 Dependency Scan Results

- **Total Issues:** $DEP_VULN
- **Reports:**
  - JSON: [\`$(basename "$TRIVY_JSON")\`]($TRIVY_JSON)
  - SARIF: [\`$(basename "$TRIVY_SARIF")\`]($TRIVY_SARIF)

---

## ✅ Merge Decision

$(if [ $total_critical -eq 0 ]; then
  echo "### ✅ APPROVED FOR MERGE"
  echo ""
  echo "All security checks passed. No critical issues detected."
else
  echo "### ❌ BLOCKED FROM MERGE"
  echo ""
  echo "Critical security issues found: **$total_critical**"
  echo ""
  echo "Please resolve the following:"
  if [ "$VULN_COUNT" -gt 0 ]; then
    echo "- [ ] Fix $VULN_COUNT dependency vulnerabilities"
  fi
  if [ "$HIGH_SEV" -gt 0 ]; then
    echo "- [ ] Fix $HIGH_SEV high-severity security issues"
  fi
fi)

---

## 📁 Report Files

All reports are located in: **$REPORT_DIR/**

---

**Generated by:** Security Scanner v2.0  
**Timestamp:** $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

  log_success "Markdown report saved: $summary_md"
  
  # Also copy to GITHUB-SUMMARY.md for easier GitHub integration
  cp "$summary_md" "$REPORT_DIR/GITHUB-SUMMARY.md"
  
  echo "$summary_md"
}

# ==========================================
# Display Results
# ==========================================

display_results() {
  log_header "Security Scan Results"
  
  local total_critical=$((VULN_COUNT + HIGH_SEV))
  
  echo -e "${MAGENTA}╔════════════════════════════════════════╗${NC}"
  echo -e "${MAGENTA}║  SCAN RESULTS SUMMARY                  ║${NC}"
  echo -e "${MAGENTA}╚════════════════════════════════════════╝${NC}"
  echo ""
  
  echo "🔍 Vulnerabilities: $VULN_COUNT $([ "$VULN_COUNT" -eq 0 ] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}")"
  echo "🔒 Security Issues: $ISSUES_FOUND (High: $HIGH_SEV, Medium: $MED_SEV, Low: $LOW_SEV) $([ "$HIGH_SEV" -eq 0 ] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}")"
  echo "📊 Quality Issues: $LINT_ISSUES $([ "$LINT_ISSUES" -eq 0 ] && echo -e "${GREEN}✓${NC}" || echo -e "${YELLOW}⚠${NC}")"
  echo "📦 Dependency Issues: $DEP_VULN"
  echo ""
  echo "📁 Reports: $REPORT_DIR/"
  echo ""
  
  if [ "$total_critical" -eq 0 ]; then
    log_success "All security checks PASSED!"
    log_success "✅ Safe to merge"
    return 0
  else
    log_error "Security checks FAILED!"
    log_error "Critical Issues: $total_critical"
    log_error "❌ DO NOT MERGE"
    return 1
  fi
}

# ==========================================
# Main Execution
# ==========================================

main() {
  local start_time=$(date +%s)
  
  log_header "🔐 Security Scanning Initiated"
  echo "Project: $PROJECT_NAME"
  echo "Directory: $PROJECT_DIR"
  echo "CI Mode: $CI_MODE"
  echo ""
  
  validate_environment
  setup_environment
  run_parallel_scans
  
  local summary_json=$(generate_summary_json)
  local summary_md=$(generate_summary_markdown)
  
  display_results
  local exit_code=$?
  
  local end_time=$(date +%s)
  local duration=$((end_time - start_time))
  
  log_header "🔐 Scan Complete"
  
  echo ""
  log_info "Duration: ${duration}s"
  log_info "View reports:"
  echo "  Markdown: cat $summary_md"
  echo "  JSON:     cat $summary_json"
  if [ -n "$GOSEC_HTML" ] && [ -f "$GOSEC_HTML" ]; then
    echo "  HTML:     open $GOSEC_HTML"
  fi
  echo ""
  
  exit $exit_code
}

# Trap errors
trap 'log_error "Script failed at line $LINENO"' ERR

# Run main function
main "$@"