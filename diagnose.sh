#!/bin/bash

# ==========================================
# Go Module & Security Scanner Diagnostic
# ==========================================

set -u

echo "🔍 Diagnosing Go Module Setup..."
echo ""

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

check_item() {
  local name=$1
  local condition=$2
  
  if eval "$condition"; then
    echo -e "${GREEN}✅${NC} $name"
    return 0
  else
    echo -e "${RED}❌${NC} $name"
    return 1
  fi
}

# ==========================================
# Check Go Installation
# ==========================================
echo -e "${BLUE}1. Go Installation${NC}"
check_item "Go installed" "command -v go &>/dev/null"
if command -v go &>/dev/null; then
  go version
fi
echo ""

# ==========================================
# Check Current Directory
# ==========================================
echo -e "${BLUE}2. Current Directory${NC}"
echo "Working directory: $(pwd)"
check_item "go.mod exists in current dir" "[ -f './go.mod' ]"
check_item "go.sum exists in current dir" "[ -f './go.sum' ]"
echo ""

# ==========================================
# Check Module Name
# ==========================================
echo -e "${BLUE}3. Module Configuration${NC}"
if [ -f "go.mod" ]; then
  MODULE_NAME=$(head -n 1 go.mod | awk '{print $2}')
  echo "Module name: $MODULE_NAME"
  echo "Go version in go.mod:"
  grep "^go " go.mod
  echo ""
  echo "Dependencies in go.mod:"
  grep "^require " go.mod -A 20 | head -10
else
  echo -e "${RED}❌${NC} go.mod not found"
fi
echo ""

# ==========================================
# Check govulncheck
# ==========================================
echo -e "${BLUE}4. Vulnerability Scanner (govulncheck)${NC}"
check_item "govulncheck installed" "command -v govulncheck &>/dev/null"

if command -v govulncheck &>/dev/null; then
  echo "Version: $(govulncheck -version 2>&1 || echo 'unknown')"
  echo ""
  echo "Testing govulncheck..."
  if GO111MODULE=on govulncheck -json ./... 2>&1 | head -20; then
    echo -e "${GREEN}✅ govulncheck works!${NC}"
  else
    echo -e "${RED}❌ govulncheck failed${NC}"
    echo "Troubleshooting:"
    echo "  1. Run: go mod tidy"
    echo "  2. Run: go mod verify"
    echo "  3. Run: govulncheck ./..."
  fi
else
  echo -e "${YELLOW}⚠️  govulncheck not installed${NC}"
  echo "Installing: go install golang.org/x/vuln/cmd/govulncheck@latest"
  go install golang.org/x/vuln/cmd/govulncheck@latest
fi
echo ""

# ==========================================
# Check gosec
# ==========================================
echo -e "${BLUE}5. Security Scanner (gosec)${NC}"
check_item "gosec installed" "command -v gosec &>/dev/null"

if command -v gosec &>/dev/null; then
  echo "Version: $(gosec --version 2>&1 | head -1)"
  echo ""
  echo "Testing gosec..."
  if gosec -fmt=json ./... 2>&1 | head -10; then
    echo -e "${GREEN}✅ gosec works!${NC}"
  else
    echo -e "${RED}❌ gosec failed${NC}"
  fi
else
  echo -e "${YELLOW}⚠️  gosec not installed${NC}"
  echo "Installing: go install github.com/securego/gosec/v2/cmd/gosec@latest"
  go install github.com/securego/gosec/v2/cmd/gosec@latest
fi
echo ""

# ==========================================
# Check golangci-lint
# ==========================================
echo -e "${BLUE}6. Code Quality (golangci-lint)${NC}"
check_item "golangci-lint installed" "command -v golangci-lint &>/dev/null"

if command -v golangci-lint &>/dev/null; then
  echo "Version: $(golangci-lint --version 2>&1 | head -1)"
else
  echo -e "${YELLOW}⚠️  golangci-lint not installed${NC}"
  echo "Installing: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi
echo ""

# ==========================================
# Check Go paths
# ==========================================
echo -e "${BLUE}7. Go Paths${NC}"
echo "GOPATH: $(go env GOPATH)"
echo "GOROOT: $(go env GOROOT)"
echo "GO111MODULE: ${GO111MODULE:-not set (auto)}"
echo ""

# ==========================================
# Summary & Fixes
# ==========================================
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Suggested Fixes (if needed):${NC}"
echo ""

if [ ! -f "go.mod" ]; then
  echo "1️⃣  Initialize Go module:"
  echo "   ${YELLOW}go mod init github.com/yourusername/yourproject${NC}"
  echo ""
fi

echo "2️⃣  Clean and verify dependencies:"
echo "   ${YELLOW}go mod tidy${NC}"
echo "   ${YELLOW}go mod verify${NC}"
echo ""

echo "3️⃣  Download dependencies:"
echo "   ${YELLOW}go mod download${NC}"
echo ""

echo "4️⃣  Force reinstall security tools:"
echo "   ${YELLOW}go install golang.org/x/vuln/cmd/govulncheck@latest${NC}"
echo "   ${YELLOW}go install github.com/securego/gosec/v2/cmd/gosec@latest${NC}"
echo ""

echo "5️⃣  Then run the security scanner:"
echo "   ${YELLOW}bash sca.sh${NC}"
echo ""

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# ==========================================
# Quick Test
# ==========================================
echo -e "${BLUE}Quick Test${NC}"
echo "Running: GO111MODULE=on govulncheck -json ./..."
echo ""

if [ -f "go.mod" ]; then
  if GO111MODULE=on govulncheck -json ./... 2>&1 | jq . > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Everything works!${NC}"
    echo ""
    echo "You can now run: ${YELLOW}bash sca.sh${NC}"
  else
    echo -e "${RED}❌ govulncheck still has issues${NC}"
    echo ""
    echo "Try these steps:"
    echo "1. cd $(pwd)"
    echo "2. go mod tidy"
    echo "3. go mod download"
    echo "4. go mod verify"
    echo "5. bash scan.sh"
  fi
else
  echo -e "${RED}❌ go.mod not found${NC}"
  echo "Cannot proceed without go.mod in: $(pwd)"
fi