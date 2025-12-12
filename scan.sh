#!/bin/bash

# ==========================================
#  Gaurav's Go Security + Lint Scanner
# ==========================================

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_NAME="DevOps Dashboard"
REPORT_DIR="security-reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo -e "${BLUE}🔍 Starting Security Scan for: ${GREEN}$PROJECT_NAME${NC}"
echo ""

# ==========================================
# Validate Go project
# ==========================================
if [ ! -f "go.mod" ]; then
  echo -e "${RED}❌ go.mod not found. Run script from project root.${NC}"
  exit 1
fi

echo -e "${GREEN}✓ go.mod found${NC}"
PROJECT_ROOT=$(pwd)

# ==========================================
# Prepare Report directory
# ==========================================
rm -rf "$REPORT_DIR"
mkdir -p "$REPORT_DIR"

echo -e "${GREEN}✓ Clean report folder created: $REPORT_DIR${NC}"

# ==========================================
# Install Tools
# ==========================================
echo -e "${BLUE}📦 Installing Go Security Tools...${NC}"

go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

export PATH="$PATH:$(go env GOPATH)/bin"

# ==========================================
# Download Dependencies
# ==========================================
go mod download
go mod verify

echo -e "${GREEN}✓ Dependencies installed${NC}"

# ==========================================
# 1️⃣ govulncheck – Dependency Vulnerability Scan
# ==========================================
echo -e "${BLUE}🔍 Running govulncheck...${NC}"

VULN_JSON="$REPORT_DIR/govulncheck-$TIMESTAMP.json"
if govulncheck -json ./... > "$VULN_JSON" 2>/dev/null; then
  VULN_STATUS="PASSED"
else
  VULN_STATUS="FAILED"
fi

VULN_COUNT=$(jq '[.finding] | length' "$VULN_JSON" 2>/dev/null || echo 0)
echo -e "${GREEN}✓ govulncheck done. Issues: ${VULN_COUNT}${NC}"

# ==========================================
# 2️⃣ gosec – Static Code Security Analysis
# ==========================================
echo -e "${BLUE}🔒 Running gosec security analysis...${NC}"

GOSEC_JSON="$REPORT_DIR/gosec-$TIMESTAMP.json"
GOSEC_HTML="$REPORT_DIR/gosec-$TIMESTAMP.html"

gosec -fmt=json -out="$GOSEC_JSON" ./... >/dev/null 2>&1 || true
gosec -fmt=html -out="$GOSEC_HTML" ./... >/dev/null 2>&1 || true

ISSUES_FOUND=$(jq -r '.Stats.found // 0' "$GOSEC_JSON" 2>/dev/null || echo 0)
echo -e "${GREEN}✓ gosec completed. Issues: ${ISSUES_FOUND}${NC}"

# ==========================================
# 3️⃣ golangci-lint – Code Quality
# ==========================================
echo -e "${BLUE}📊 Running golangci-lint...${NC}"

LINT_JSON="$REPORT_DIR/golangci-$TIMESTAMP.json"
golangci-lint run --out-format json ./... > "$LINT_JSON" 2>&1 || true

LINT_ISSUES=$(jq '[.Issues[]] | length' "$LINT_JSON" 2>/dev/null || echo 0)
echo -e "${GREEN}✓ Linting done. Issues: ${LINT_ISSUES}${NC}"

# ==========================================
# Summary Report
# ==========================================
TOTAL_CRITICAL=$((ISSUES_FOUND + VULN_COUNT))

SUMMARY="$REPORT_DIR/summary-$TIMESTAMP.txt"

echo "==================== SECURITY SUMMARY ====================" > "$SUMMARY"
echo "Project: $PROJECT_NAME" >> "$SUMMARY"
echo "Time: $TIMESTAMP" >> "$SUMMARY"
echo "" >> "$SUMMARY"
echo "govulncheck Issues : $VULN_COUNT" >> "$SUMMARY"
echo "gosec Issues       : $ISSUES_FOUND" >> "$SUMMARY"
echo "Lint Issues        : $LINT_ISSUES" >> "$SUMMARY"
echo "" >> "$SUMMARY"
echo "Total Critical     : $TOTAL_CRITICAL" >> "$SUMMARY"
echo "==========================================================" >> "$SUMMARY"

echo ""
echo -e "${BLUE}📄 Summary Report: ${GREEN}$SUMMARY${NC}"

# ==========================================
# Exit with correct status code
# ==========================================
if [ "$TOTAL_CRITICAL" -gt 0 ]; then
  echo -e "${RED}❌ Critical issues found. Failing pipeline.${NC}"
  exit 1
else
  echo -e "${GREEN}✅ No critical issues. Scan successful.${NC}"
  exit 0
fi
