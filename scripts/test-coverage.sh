#!/bin/bash
# Amanah Test Coverage Script
# Generates detailed coverage reports for all services

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COVERAGE_DIR="coverage"
MIN_COVERAGE=${MIN_COVERAGE:-70}

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Amanah Test Coverage Report${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Create coverage directory
mkdir -p "$COVERAGE_DIR"

# Services to test
SERVICES=(
    "services/authentication"
    "services/transaction"
    "services/account"
    "services/ledger"
    "services/notification"
    "services/api-gateway"
)

# Libraries to test
LIBS=(
    "libs/auth"
    "libs/middleware"
    "libs/resilience"
    "libs/gateway"
    "libs/logging"
    "libs/metrics"
    "libs/testing"
    "libs/cache"
    "libs/config"
    "libs/database"
)

# Run tests with coverage for each service
echo -e "${YELLOW}Running service tests...${NC}"
echo ""

for service in "${SERVICES[@]}"; do
    name=$(basename "$service")
    echo -e "${BLUE}Testing: $service${NC}"

    if go test -coverprofile="$COVERAGE_DIR/${name}.out" "./$service/..." 2>/dev/null; then
        coverage=$(go tool cover -func="$COVERAGE_DIR/${name}.out" 2>/dev/null | tail -1 | awk '{print $3}' | sed 's/%//')
        if (( $(echo "$coverage >= $MIN_COVERAGE" | bc -l) )); then
            echo -e "  ${GREEN}Coverage: ${coverage}%${NC}"
        else
            echo -e "  ${RED}Coverage: ${coverage}% (below ${MIN_COVERAGE}% threshold)${NC}"
        fi
    else
        echo -e "  ${YELLOW}No tests found or tests failed${NC}"
    fi
done

echo ""
echo -e "${YELLOW}Running library tests...${NC}"
echo ""

for lib in "${LIBS[@]}"; do
    name=$(basename "$lib")
    echo -e "${BLUE}Testing: $lib${NC}"

    if go test -coverprofile="$COVERAGE_DIR/lib-${name}.out" "./$lib/..." 2>/dev/null; then
        coverage=$(go tool cover -func="$COVERAGE_DIR/lib-${name}.out" 2>/dev/null | tail -1 | awk '{print $3}' | sed 's/%//')
        if (( $(echo "$coverage >= $MIN_COVERAGE" | bc -l) )); then
            echo -e "  ${GREEN}Coverage: ${coverage}%${NC}"
        else
            echo -e "  ${RED}Coverage: ${coverage}% (below ${MIN_COVERAGE}% threshold)${NC}"
        fi
    else
        echo -e "  ${YELLOW}No tests found or tests failed${NC}"
    fi
done

# Run full coverage
echo ""
echo -e "${YELLOW}Generating combined coverage report...${NC}"

go test -coverprofile="$COVERAGE_DIR/coverage.out" -covermode=atomic ./... 2>/dev/null || true

if [ -f "$COVERAGE_DIR/coverage.out" ]; then
    # Generate HTML report
    go tool cover -html="$COVERAGE_DIR/coverage.out" -o "$COVERAGE_DIR/coverage.html"

    # Generate function-level report
    go tool cover -func="$COVERAGE_DIR/coverage.out" > "$COVERAGE_DIR/coverage.txt"

    # Get total coverage
    total_coverage=$(tail -1 "$COVERAGE_DIR/coverage.txt" | awk '{print $3}' | sed 's/%//')

    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}  Coverage Summary${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""

    if (( $(echo "$total_coverage >= $MIN_COVERAGE" | bc -l) )); then
        echo -e "  Total Coverage: ${GREEN}${total_coverage}%${NC}"
    else
        echo -e "  Total Coverage: ${RED}${total_coverage}%${NC} (target: ${MIN_COVERAGE}%)"
    fi

    echo ""
    echo "  Reports generated:"
    echo "    - HTML: $COVERAGE_DIR/coverage.html"
    echo "    - Text: $COVERAGE_DIR/coverage.txt"
    echo ""

    # Show low coverage files
    echo -e "${YELLOW}Files with lowest coverage:${NC}"
    grep -v "^total:" "$COVERAGE_DIR/coverage.txt" | \
        awk '{print $3, $1}' | \
        sed 's/%//' | \
        sort -n | \
        head -10 | \
        while read coverage file; do
            if (( $(echo "$coverage < $MIN_COVERAGE" | bc -l) )); then
                echo -e "  ${RED}${coverage}%${NC} - $file"
            else
                echo -e "  ${GREEN}${coverage}%${NC} - $file"
            fi
        done

    echo ""
else
    echo -e "${RED}Failed to generate coverage report${NC}"
    exit 1
fi

echo -e "${GREEN}Coverage analysis complete!${NC}"
