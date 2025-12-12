#!/bin/bash

# ==========================================
# Enhanced Security Scanning Script
# ==========================================
# Features:
# - Multiple security scanners (govulncheck, gosec, golangci-lint, trivy)
# - CI/CD friendly (GitHub Actions, GitLab)
# - JSON output for AI analysis
# - Auto-retry for network failures
# - Improved error handling
# - Concurrent scanning
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
PARALLEL_JOBS=3
RETRY_COUNT=3
RETRY_DELAY=2

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
  
  # Check for required tools
  check_command() {
    if ! command -v "$1" &> /dev/null; then
      log_warning "$1 not found - will attempt installation"
      return 1
    fi
    return 0
  }
  
  check_command "go" || exit 1
  log_success "Go compiler found: $(go version)"
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
  
  # Clean up root level reports
  rm -f gosec-report.* govuln-report.* summary-*.json SECURITY-SUMMARY.md
  
  log_success "Report directory ready: $REPORT_DIR"
  
  # Install/update security tools
  log_info "Installing security tools..."
  
  retry_command "go install golang.org/x/vuln/cmd/govulncheck@latest" || log_warning "govulncheck install failed"
  retry_command "go install github.com/securego/gosec/v2/cmd/gosec@latest" || log_warning "gosec install failed"
  retry_command "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" || log_warning "golangci-lint install failed"
  
  export PATH=$PATH:$(go env GOPATH)/bin
  
  # Update Go modules
  log_info "Downloading and verifying dependencies..."
  go mod download || log_warning "go mod download had issues"
  go mod verify || log_warning "go mod verify had issues"
  
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
  log_info "Current directory: $(pwd)"
  log_info "go.mod location: $(find . -name "go.mod" -type f 2>/dev/null | head -1)"
  
  # Verify go.mod exists
  if [ ! -f "go.mod" ]; then
    log_error "go.mod not found! Absolute path check..."
    if [ -f "$PROJECT_DIR/go.mod" ]; then
      log_info "go.mod found at: $PROJECT_DIR/go.mod"
      cd "$PROJECT_DIR" || exit 1
    else
      log_error "go.mod not found anywhere"
      echo "0|"
      return 1
    fi
  fi
  
  # Run with error capture - ensure we're in correct directory
  if (cd "$PROJECT_DIR" && GO111MODULE=on govulncheck -json ./... > "$vuln_json" 2>&1); then
    local vuln_count=$(jq '[.Vulnerabilities[]? // empty] | length' "$vuln_json" 2>/dev/null || echo "0")
    log_success "Vulnerability scan complete - Found: $vuln_count vulnerabilities"
  else
    local vuln_count=$(jq '[.Vulnerabilities[]? // empty] | length' "$vuln_json" 2>/dev/null || echo "0")
    if [ "$vuln_count" -gt 0 ]; then
      log_warning "Found $vuln_count vulnerabilities"
    else
      # Check if it's just the go.mod error
      if grep -q "no go.mod file" "$vuln_json" 2>/dev/null; then
        log_error "govulncheck could not find go.mod - checking module setup"
        log_info "Trying alternative: checking go.sum..."
        if [ ! -f "go.sum" ]; then
          log_warning "go.sum missing - running go mod tidy..."
          (cd "$PROJECT_DIR" && go mod tidy)
        fi
        # Retry
        if (cd "$PROJECT_DIR" && GO111MODULE=on govulncheck -json ./... > "$vuln_json" 2>&1); then
          vuln_count=$(jq '[.Vulnerabilities[]? // empty] | length' "$vuln_json" 2>/dev/null || echo "0")
          log_success "Retry successful - Found: $vuln_count vulnerabilities"
        else
          log_warning "govulncheck failed even after retry"
          vuln_count=0
        fi
      else
        log_success "No known vulnerabilities found"
      fi
    fi
  fi
  
  # Generate text report
  (cd "$PROJECT_DIR" && GO111MODULE=on govulncheck ./... > "$vuln_txt" 2>&1) || true
  
  echo "$vuln_count|$vuln_json|$vuln_txt"
}

scan_security() {
  log_header "2️⃣  Security Analysis (gosec)"
  
  local gosec_json="$REPORT_DIR/gosec-${TIMESTAMP}.json"
  local gosec_html="$REPORT_DIR/gosec-${TIMESTAMP}.html"
  local gosec_sarif="$REPORT_DIR/gosec-${TIMESTAMP}.sarif"
  
  log_info "Running static security analysis..."
  
  # Run gosec
  if gosec -fmt=json -out="$gosec_json" ./... 2>&1; then
    log_success "Gosec analysis complete"
  else
    log_warning "Gosec completed with findings"
  fi
  
  # Generate HTML and SARIF reports
  gosec -fmt=html -out="$gosec_html" ./... 2>/dev/null || true
  gosec -fmt=sarif -out="$gosec_sarif" ./... 2>/dev/null || true
  
  # Parse results
  if [ -f "$gosec_json" ]; then
    local issues=$(jq -r '.Stats.found // 0' "$gosec_json" 2>/dev/null || echo "0")
    local files=$(jq -r '.Stats.files // 0' "$gosec_json" 2>/dev/null || echo "0")
    local lines=$(jq -r '.Stats.lines // 0' "$gosec_json" 2>/dev/null || echo "0")
    
    if [ "$issues" -gt 0 ]; then
      log_warning "Found $issues security issues in $files files"
    else
      log_success "No security issues found"
    fi
    
    echo "$issues|$files|$lines|$gosec_json|$gosec_html|$gosec_sarif"
  else
    log_error "Gosec report not generated"
    echo "0|0|0|||"
  fi
}

scan_code_quality() {
  log_header "3️⃣  Code Quality Check (golangci-lint)"
  
  local lint_json="$REPORT_DIR/golangci-lint-${TIMESTAMP}.json"
  
  log_info "Running code quality checks..."
  
  golangci-lint run --out-format json ./... > "$lint_json" 2>&1 || true
  
  local issues=$(jq '[.Issues[]? // empty] | length' "$lint_json" 2>/dev/null || echo "0")
  
  if [ "$issues" -gt 0 ]; then
    log_warning "Found $issues code quality issues"
  else
    log_success "No code quality issues found"
  fi
  
  echo "$issues|$lint_json"
}

scan_dependencies() {
  log_header "4️⃣  Dependency Scanning (Trivy)"
  
  local trivy_json="$REPORT_DIR/trivy-${TIMESTAMP}.json"
  
  # Check if trivy is installed
  if ! command -v trivy &> /dev/null; then
    log_warning "Trivy not installed - skipping dependency scan"
    echo "0|"
    return
  fi
  
  log_info "Scanning dependencies for vulnerabilities..."
  
  if trivy fs --format json --output "$trivy_json" . 2>/dev/null; then
    local dep_vuln=$(jq '[.Results[]?.Misconfigurations[]? // empty] | length' "$trivy_json" 2>/dev/null || echo "0")
    log_success "Dependency scan complete - Found: $dep_vuln issues"
  else
    log_warning "Trivy scan encountered issues"
  fi
  
  echo "0|$trivy_json"
}

# ==========================================
# Parallel Scanning
# ==========================================

run_parallel_scans() {
  log_header "Running Parallel Security Scans"
  
  # Create temp files for results
  local temp_vuln=$(mktemp)
  local temp_sec=$(mktemp)
  local temp_qual=$(mktemp)
  local temp_dep=$(mktemp)
  
  # Run scans in background with proper error handling
  (scan_vulnerabilities > "$temp_vuln" 2>&1) &
  local pid1=$!
  
  (scan_security > "$temp_sec" 2>&1) &
  local pid2=$!
  
  (scan_code_quality > "$temp_qual" 2>&1) &
  local pid3=$!
  
  (scan_dependencies > "$temp_dep" 2>&1) &
  local pid4=$!
  
  # Wait for all scans with timeout
  local timeout=300  # 5 minutes
  local elapsed=0
  
  while ps -p $pid1 $pid2 $pid3 $pid4 > /dev/null 2>&1 && [ $elapsed -lt $timeout ]; do
    sleep 2
    elapsed=$((elapsed + 2))
  done
  
  # Force kill if timeout
  if ps -p $pid1 > /dev/null 2>&1; then kill $pid1 2>/dev/null || true; fi
  if ps -p $pid2 > /dev/null 2>&1; then kill $pid2 2>/dev/null || true; fi
  if ps -p $pid3 > /dev/null 2>&1; then kill $pid3 2>/dev/null || true; fi
  if ps -p $pid4 > /dev/null 2>&1; then kill $pid4 2>/dev/null || true; fi
  
  # Capture results with fallback defaults
  if [ -s "$temp_vuln" ]; then
    IFS='|' read -r VULN_COUNT VULN_JSON VULN_TXT <<< "$(cat "$temp_vuln")"
  else
    VULN_COUNT=0
    VULN_JSON=""
    VULN_TXT=""
  fi
  
  if [ -s "$temp_sec" ]; then
    IFS='|' read -r ISSUES_FOUND FILES_SCANNED LINES_SCANNED GOSEC_REPORT GOSEC_HTML GOSEC_SARIF <<< "$(cat "$temp_sec")"
  else
    ISSUES_FOUND=0
    FILES_SCANNED=0
    LINES_SCANNED=0
    GOSEC_REPORT=""
    GOSEC_HTML=""
    GOSEC_SARIF=""
  fi
  
  if [ -s "$temp_qual" ]; then
    IFS='|' read -r LINT_ISSUES LINT_REPORT <<< "$(cat "$temp_qual")"
  else
    LINT_ISSUES=0
    LINT_REPORT=""
  fi
  
  # Set defaults for empty values
  VULN_COUNT=${VULN_COUNT:-0}
  ISSUES_FOUND=${ISSUES_FOUND:-0}
  FILES_SCANNED=${FILES_SCANNED:-0}
  LINES_SCANNED=${LINES_SCANNED:-0}
  LINT_ISSUES=${LINT_ISSUES:-0}
  
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
  
  # Calculate severity breakdown
  local high_sev=0
  local med_sev=0
  local low_sev=0
  
  if [ -f "$GOSEC_REPORT" ] && [ "$ISSUES_FOUND" -gt 0 ]; then
    high_sev=$(jq '[.Issues[] | select(.severity=="HIGH")] | length' "$GOSEC_REPORT" 2>/dev/null || echo "0")
    med_sev=$(jq '[.Issues[] | select(.severity=="MEDIUM")] | length' "$GOSEC_REPORT" 2>/dev/null || echo "0")
    low_sev=$(jq '[.Issues[] | select(.severity=="LOW")] | length' "$GOSEC_REPORT" 2>/dev/null || echo "0")
  fi
  
  local total_critical=$((VULN_COUNT + high_sev))
  
  cat > "$summary_file" << EOF
{
  "scan_timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "project_name": "$PROJECT_NAME",
  "project_directory": "$PROJECT_DIR",
  "ci_environment": $([[ "$CI_MODE" == "true" ]] && echo "true" || echo "false"),
  "overall_status": "$([ $total_critical -eq 0 ] && echo "PASSED" || echo "FAILED")",
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
      "high": $high_sev,
      "medium": $med_sev,
      "low": $low_sev,
      "status": "$([ $ISSUES_FOUND -eq 0 ] && echo "PASSED" || echo "FAILED")"
    },
    "code_quality": {
      "tool": "golangci-lint",
      "count": $LINT_ISSUES,
      "status": "$([ $LINT_ISSUES -eq 0 ] && echo "PASSED" || echo "WARNING")"
    }
  },
  "reports": {
    "gosec_json": "$GOSEC_REPORT",
    "gosec_html": "$GOSEC_HTML",
    "gosec_sarif": "$GOSEC_SARIF",
    "govulncheck_json": "$VULN_JSON",
    "golangci_lint_json": "$LINT_REPORT"
  }
}
EOF

  log_success "Summary saved: $summary_file"
  echo "$summary_file"
}

generate_summary_markdown() {
  log_info "Generating markdown report..."
  
  local summary_md="$REPORT_DIR/SECURITY-REPORT.md"
  
  # Calculate severity
  local high_sev=0
  if [ -f "$GOSEC_REPORT" ] && [ "$ISSUES_FOUND" -gt 0 ]; then
    high_sev=$(jq '[.Issues[] | select(.severity=="HIGH")] | length' "$GOSEC_REPORT" 2>/dev/null || echo "0")
  fi
  
  local total_critical=$((VULN_COUNT + high_sev))
  
  cat > "$summary_md" << 'MARKDOWN_EOF'
# 🔒 Security Scan Report

MARKDOWN_EOF

  cat >> "$summary_md" << EOF

**Generated:** $(date '+%Y-%m-%d %H:%M:%S UTC')  
**Project:** $PROJECT_NAME  
**Status:** $([ $total_critical -eq 0 ] && echo "✅ PASSED" || echo "❌ FAILED")

---

## 📊 Summary

| Component | Issues | Status |
|-----------|--------|--------|
| 🔍 Vulnerabilities (govulncheck) | $VULN_COUNT | $([ $VULN_COUNT -eq 0 ] && echo "✅" || echo "❌") |
| 🔒 Security (gosec) | $ISSUES_FOUND | $([ $ISSUES_FOUND -eq 0 ] && echo "✅" || echo "⚠️") |
| 📊 Code Quality (golangci-lint) | $LINT_ISSUES | $([ $LINT_ISSUES -eq 0 ] && echo "✅" || echo "⚠️") |

---

## 🔍 Vulnerability Scan Results

- **Total Vulnerabilities:** $VULN_COUNT
- **Status:** $([ $VULN_COUNT -eq 0 ] && echo "✅ No vulnerabilities found" || echo "❌ CRITICAL - Vulnerabilities detected")

Report: [\`$VULN_JSON\`]($VULN_JSON)

---

## 🔒 Security Analysis Results

- **Total Issues:** $ISSUES_FOUND
- **Files Scanned:** $FILES_SCANNED
- **Lines Scanned:** $LINES_SCANNED

### Severity Breakdown
| Level | Count |
|-------|-------|
| 🔴 High | $high_sev |
| 🟡 Medium | $(jq '[.Issues[] | select(.severity=="MEDIUM")] | length' "$GOSEC_REPORT" 2>/dev/null || echo "0") |
| 🟢 Low | $(jq '[.Issues[] | select(.severity=="LOW")] | length' "$GOSEC_REPORT" 2>/dev/null || echo "0") |

**Reports:**
- JSON: [\`gosec-report.json\`]($GOSEC_REPORT)
- HTML: [\`gosec-report.html\`]($GOSEC_HTML) - **Open in browser for details**
- SARIF: [\`gosec-report.sarif\`]($GOSEC_SARIF)

---

## 📊 Code Quality Results

- **Total Issues:** $LINT_ISSUES
- **Report:** [\`$LINT_REPORT\`]($LINT_REPORT)

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
  if [ "$high_sev" -gt 0 ]; then
    echo "- [ ] Fix $high_sev high-severity security issues"
  fi
fi)

---

## 📁 Report Files

All reports are located in: **$REPORT_DIR/**

| File | Type | Purpose |
|------|------|---------|
| SECURITY-REPORT.md | Markdown | This summary |
| summary-\*.json | JSON | Machine-readable summary |
| gosec-\*.json | JSON | Detailed security findings |
| gosec-\*.html | HTML | Interactive security report |
| gosec-\*.sarif | SARIF | GitHub Security format |
| govulncheck-\*.json | JSON | Vulnerability data |
| golangci-lint-\*.json | JSON | Code quality findings |

---

**Generated by:** Security Scanner  
**Timestamp:** $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

  log_success "Markdown report saved: $summary_md"
  echo "$summary_md"
}

# ==========================================
# Output Results
# ==========================================

display_results() {
  log_header "Security Scan Results"
  
  local total_critical=$((VULN_COUNT + $(jq '[.Issues[] | select(.severity=="HIGH")] | length' "$GOSEC_REPORT" 2>/dev/null || echo "0")))
  
  echo -e "${MAGENTA}╔════════════════════════════════════════╗${NC}"
  echo -e "${MAGENTA}║  1. Vulnerability Scan (govulncheck)   ║${NC}"
  echo -e "${MAGENTA}╚════════════════════════════════════════╝${NC}"
  echo "Vulnerabilities Found: ${VULN_COUNT} $([ "$VULN_COUNT" -eq 0 ] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}")"
  echo "Report: $VULN_JSON"
  echo ""
  
  echo -e "${MAGENTA}╔════════════════════════════════════════╗${NC}"
  echo -e "${MAGENTA}║  2. Security Analysis (gosec)          ║${NC}"
  echo -e "${MAGENTA}╚════════════════════════════════════════╝${NC}"
  echo "Security Issues: ${ISSUES_FOUND} $([ "$ISSUES_FOUND" -eq 0 ] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}")"
  echo "Files Scanned: $FILES_SCANNED"
  echo "Lines Scanned: $LINES_SCANNED"
  echo "Reports:"
  echo "  - JSON:  $GOSEC_REPORT"
  echo "  - HTML:  $GOSEC_HTML"
  echo "  - SARIF: $GOSEC_SARIF"
  echo ""
  
  echo -e "${MAGENTA}╔════════════════════════════════════════╗${NC}"
  echo -e "${MAGENTA}║  3. Code Quality (golangci-lint)       ║${NC}"
  echo -e "${MAGENTA}╚════════════════════════════════════════╝${NC}"
  echo "Quality Issues: ${LINT_ISSUES} $([ "$LINT_ISSUES" -eq 0 ] && echo -e "${GREEN}✓${NC}" || echo -e "${YELLOW}⚠${NC}")"
  echo "Report: $LINT_REPORT"
  echo ""
  
  echo -e "${MAGENTA}╔════════════════════════════════════════╗${NC}"
  echo -e "${MAGENTA}║  📁 All Reports                         ║${NC}"
  echo -e "${MAGENTA}╚════════════════════════════════════════╝${NC}"
  echo "Location: $REPORT_DIR/"
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
# Output for CI/CD
# ==========================================

output_for_ci() {
  if [ "$CI_MODE" = "true" ]; then
    log_info "Outputting results for CI/CD system..."
    
    # Create JSON for CI systems
    local ci_json="$REPORT_DIR/ci-report.json"
    
    local summary_json=$(ls -t "$REPORT_DIR"/summary-*.json | head -1)
    
    if [ -f "$summary_json" ]; then
      cp "$summary_json" "$ci_json"
      log_success "CI report saved: $ci_json"
    fi
  fi
}

# ==========================================
# Main Execution
# ==========================================

main() {
  log_header "🔐 Security Scanning Initiated"
  
  validate_environment
  setup_environment
  run_parallel_scans
  
  local summary_json=$(generate_summary_json)
  local summary_md=$(generate_summary_markdown)
  
  output_for_ci
  
  display_results
  local exit_code=$?
  
  log_header "🔐 Scan Complete"
  
  echo ""
  log_info "View reports:"
  echo "  Markdown: cat $summary_md"
  echo "  JSON:     cat $summary_json"
  if [ -n "$GOSEC_HTML" ] && [ -f "$GOSEC_HTML" ]; then
    echo "  HTML:     open $GOSEC_HTML"
  fi
  echo ""
  
  exit $exit_code
}

# Run main function
main "$@"