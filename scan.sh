#!/bin/bash

# ==========================================
# Fast Security Scanning Script
# ==========================================

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration variables
REPORT_DIR="security-reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
PROJECT_NAME="${PROJECT_NAME:-Go Project}"

# Enable strict mode
set -euo pipefail

# ==========================================
# Validation
# ==========================================

# Check if go.mod exists in current directory
if [ ! -f "go.mod" ]; then
  echo -e "${RED}❌ Error: go.mod not found in current directory${NC}"
  echo -e "${YELLOW}Please run this script from your project root directory where go.mod is located.${NC}"
  echo -e "${BLUE}Current directory: $(pwd)${NC}"
  exit 1
fi

echo -e "${GREEN}✓ Found go.mod in $(pwd)${NC}"

# Get the absolute path of the project directory and store it
PROJECT_DIR="$(pwd)"
echo -e "${BLUE}Project directory: $PROJECT_DIR${NC}"

# Function to ensure we're in the project directory
ensure_project_dir() {
  if [ "$(pwd)" != "$PROJECT_DIR" ]; then
    echo -e "${YELLOW}⚠️  Directory changed, returning to project root: $PROJECT_DIR${NC}"
    cd "$PROJECT_DIR" || exit 1
  fi
}

# ==========================================
# Setup
# ==========================================

echo -e "${BLUE}📦 Setting up security scanning environment...${NC}"

# Ensure we're in project directory
ensure_project_dir

# Remove old report directory completely and create fresh one
echo -e "${BLUE}🧹 Cleaning up old reports...${NC}"
rm -rf "$REPORT_DIR"
mkdir -p "$REPORT_DIR"

# Clean up old reports from root directory
rm -f gosec-report.json govuln-report.txt gosec-report.html
echo -e "${GREEN}✓ Cleanup complete - fresh start${NC}"

# Install security tools
echo -e "${BLUE}🔧 Installing security tools...${NC}"
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

export PATH=$PATH:$(go env GOPATH)/bin

# ==========================================
# Dependency Download
# ==========================================

echo -e "${BLUE}📥 Downloading dependencies...${NC}"
ensure_project_dir
go mod download
go mod verify

# ==========================================
# Vulnerability Scanning
# ==========================================

echo -e "${BLUE}🔍 Running govulncheck (vulnerability scanning)...${NC}"
VULN_REPORT="$REPORT_DIR/govulncheck-${TIMESTAMP}.txt"
VULN_JSON="$REPORT_DIR/govulncheck-${TIMESTAMP}.json"

# Ensure we're in the project directory before running govulncheck
ensure_project_dir
echo -e "${BLUE}Running govulncheck from: $(pwd)${NC}"

# Verify go.mod exists right before running
if [ ! -f "go.mod" ]; then
  echo -e "${RED}❌ Error: go.mod disappeared! Current directory: $(pwd)${NC}"
  exit 1
fi

# Run govulncheck with explicit module mode
if GO111MODULE=on govulncheck -json ./... > "$VULN_JSON" 2>&1; then
  VULN_STATUS="✅ PASSED"
  VULN_COUNT=0
else
  VULN_STATUS="⚠️  VULNERABILITIES FOUND"
  VULN_COUNT=$(jq '[.finding] | length' "$VULN_JSON" 2>/dev/null | tr -d '\n' || echo "0")
  VULN_COUNT=${VULN_COUNT:-0}
fi

# Generate text report
ensure_project_dir
GO111MODULE=on govulncheck ./... > "$VULN_REPORT" 2>&1 || true

echo -e "${GREEN}✓ Vulnerability scan complete${NC}"

# ==========================================
# Static Security Analysis
# ==========================================

echo -e "${BLUE}🔒 Running gosec (static security analysis)...${NC}"
GOSEC_REPORT="$REPORT_DIR/gosec-${TIMESTAMP}.json"
GOSEC_HTML="$REPORT_DIR/gosec-${TIMESTAMP}.html"

# Ensure we're in project directory
ensure_project_dir

# Run gosec and capture output
if gosec -fmt=json -out="$GOSEC_REPORT" ./... 2>&1; then
  echo -e "${GREEN}✓ Gosec completed successfully${NC}"
else
  echo -e "${YELLOW}⚠️  Gosec completed with findings${NC}"
fi

# Generate HTML report separately
ensure_project_dir
gosec -fmt=html -out="$GOSEC_HTML" ./... 2>/dev/null || true

# Parse gosec results only if file exists
if [ -f "$GOSEC_REPORT" ]; then
  ISSUES_FOUND=$(jq -r '.Stats.found // 0' "$GOSEC_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
  FILES_SCANNED=$(jq -r '.Stats.files // 0' "$GOSEC_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
  LINES_SCANNED=$(jq -r '.Stats.lines // 0' "$GOSEC_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
  ISSUES_FOUND=${ISSUES_FOUND:-0}
  FILES_SCANNED=${FILES_SCANNED:-0}
  LINES_SCANNED=${LINES_SCANNED:-0}
else
  echo -e "${RED}❌ Gosec report not generated${NC}"
  ISSUES_FOUND=0
  FILES_SCANNED=0
  LINES_SCANNED=0
fi

echo -e "${GREEN}✓ Static analysis complete${NC}"

# ==========================================
# Code Quality Check
# ==========================================

echo -e "${BLUE}📊 Running golangci-lint (code quality)...${NC}"
LINT_REPORT="$REPORT_DIR/golangci-lint-${TIMESTAMP}.json"

# Ensure we're in project directory
ensure_project_dir

golangci-lint run --out-format json ./... > "$LINT_REPORT" 2>&1 || true
LINT_ISSUES=$(jq '[.Issues[]] | length' "$LINT_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
LINT_ISSUES=${LINT_ISSUES:-0}

echo -e "${GREEN}✓ Code quality check complete${NC}"

# ==========================================
# Generate Summary Report
# ==========================================

SUMMARY_FILE="$REPORT_DIR/summary-${TIMESTAMP}.json"

# Build severity breakdown JSON
if [ "$ISSUES_FOUND" -gt 0 ] && [ -f "$GOSEC_REPORT" ]; then
  HIGH_SEV=$(jq '[.Issues[] | select(.severity=="HIGH")] | length' "$GOSEC_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
  MEDIUM_SEV=$(jq '[.Issues[] | select(.severity=="MEDIUM")] | length' "$GOSEC_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
  LOW_SEV=$(jq '[.Issues[] | select(.severity=="LOW")] | length' "$GOSEC_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
  HIGH_SEVER=${HIGH_SEV:-0}
  MEDIUM_SEVER=${MEDIUM_SEV:-0}
  LOW_SEVER=${LOW_SEV:-0}
else
  HIGH_SEVER=0
  MEDIUM_SEVER=0
  LOW_SEVER=0
fi

# Determine exit code early for summary
if [ "$ISSUES_FOUND" -gt 0 ] && [ -f "$GOSEC_REPORT" ]; then
  HIGH_SEVERITY=$(jq '[.Issues[] | select(.severity=="HIGH")] | length' "$GOSEC_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
  HIGH_SEVERITY=${HIGH_SEVERITY:-0}
  TOTAL_CRITICAL=$((HIGH_SEVERITY + VULN_COUNT))
else
  TOTAL_CRITICAL=$VULN_COUNT
fi

if [ $TOTAL_CRITICAL -gt 0 ]; then
  EXIT_CODE=1
else
  EXIT_CODE=0
fi

ensure_project_dir

cat > "$SUMMARY_FILE" << EOF
{
  "scan_timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "project": "$PROJECT_NAME",
  "project_directory": "$PROJECT_DIR",
  "overall_status": "$([ $EXIT_CODE -eq 0 ] && echo "PASSED" || echo "FAILED")",
  "scan_tools": {
    "1_vulnerability_scan": {
      "tool": "govulncheck",
      "description": "Checks Go dependencies for known vulnerabilities",
      "status": "$VULN_STATUS",
      "vulnerabilities_found": $VULN_COUNT,
      "report_file": "$VULN_REPORT",
      "json_report": "$VULN_JSON"
    },
    "2_security_analysis": {
      "tool": "gosec",
      "description": "Static security analysis of Go code",
      "issues_found": $ISSUES_FOUND,
      "files_scanned": $FILES_SCANNED,
      "lines_scanned": $LINES_SCANNED,
      "severity_breakdown": {
        "high": $HIGH_SEVER,
        "medium": $MEDIUM_SEVER,
        "low": $LOW_SEVER
      },
      "json_report": "$GOSEC_REPORT",
      "html_report": "$GOSEC_HTML"
    },
    "3_code_quality": {
      "tool": "golangci-lint",
      "description": "Code quality and linting checks",
      "issues_found": $LINT_ISSUES,
      "report_file": "$LINT_REPORT"
    }
  }
}
EOF

# ==========================================
# Determine Overall Status
# ==========================================

SEVERITY_BREAKDOWN=""

if [ "$ISSUES_FOUND" -gt 0 ] && [ -f "$GOSEC_REPORT" ]; then
  HIGH_SEVERITY=$(jq '[.Issues[] | select(.severity=="HIGH")] | length' "$GOSEC_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
  MEDIUM_SEVERITY=$(jq '[.Issues[] | select(.severity=="MEDIUM")] | length' "$GOSEC_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
  LOW_SEVERITY=$(jq '[.Issues[] | select(.severity=="LOW")] | length' "$GOSEC_REPORT" 2>/dev/null | tr -d '\n' || echo "0")
  HIGH_SEVERITY=${HIGH_SEVERITY:-0}
  MEDIUM_SEVERITY=${MEDIUM_SEVERITY:-0}
  LOW_SEVERITY=${LOW_SEVERITY:-0}
  
  SEVERITY_BREAKDOWN="🔴 High: $HIGH_SEVERITY | 🟡 Medium: $MEDIUM_SEVERITY | 🟢 Low: $LOW_SEVERITY"
else
  SEVERITY_BREAKDOWN="No issues found"
fi

# ==========================================
# Generate Console Report
# ==========================================

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}         SECURITY SCAN SUMMARY${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "Project:          $PROJECT_NAME"
echo -e "Directory:        $PROJECT_DIR"
echo -e "Timestamp:        $(date '+%Y-%m-%d %H:%M:%S')"
echo ""
echo -e "${BLUE}╔═══════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║        1. VULNERABILITY SCAN (govulncheck) ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════╝${NC}"
echo -e "Status:           $VULN_STATUS"
echo -e "Vulnerabilities:  ${VULN_COUNT} $([ $VULN_COUNT -gt 0 ] && echo -e "${RED}⚠️${NC}" || echo -e "${GREEN}✓${NC}")"
echo -e "Report:           $VULN_REPORT"
echo ""
echo -e "${BLUE}╔═══════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║        2. SECURITY ANALYSIS (gosec)        ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════╝${NC}"
echo -e "Security Issues:  ${ISSUES_FOUND} $([ $ISSUES_FOUND -gt 0 ] && echo -e "${RED}⚠️${NC}" || echo -e "${GREEN}✓${NC}")"
if [ "$ISSUES_FOUND" -gt 0 ]; then
  echo -e "Severity:         $SEVERITY_BREAKDOWN"
fi
echo -e "Files Scanned:    $FILES_SCANNED"
echo -e "Lines Scanned:    $LINES_SCANNED"
echo -e "JSON Report:      $GOSEC_REPORT"
echo -e "HTML Report:      $GOSEC_HTML"
echo ""
echo -e "${BLUE}╔═══════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║        3. CODE QUALITY (golangci-lint)     ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════╝${NC}"
echo -e "Quality Issues:   $LINT_ISSUES $([ $LINT_ISSUES -gt 0 ] && echo -e "${YELLOW}⚠️${NC}" || echo -e "${GREEN}✓${NC}")"
echo -e "Report:           $LINT_REPORT"
echo ""
echo -e "${GREEN}📁 Summary Report: $SUMMARY_FILE${NC}"
echo ""

if [ $EXIT_CODE -eq 0 ]; then
  echo -e "${GREEN}✅ All security checks passed!${NC}"
else
  echo -e "${RED}❌ Critical security issues detected!${NC}"
  echo ""
  if [ "$VULN_COUNT" -gt 0 ]; then
    echo -e "${RED}   • $VULN_COUNT vulnerabilities found in dependencies${NC}"
  fi
  if [ "$ISSUES_FOUND" -gt 0 ]; then
    echo -e "${RED}   • $ISSUES_FOUND security issues found in code${NC}"
  fi
  echo ""
  echo -e "${YELLOW}💡 Open the HTML report for detailed findings:${NC}"
  echo -e "   open $GOSEC_HTML"
fi

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

exit $EXIT_CODE