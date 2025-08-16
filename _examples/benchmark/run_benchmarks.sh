#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color
BOLD='\033[1m'

echo -e "${BOLD}${BLUE}==================================================================${NC}"
echo -e "${BOLD}${BLUE}          HTTP Framework Benchmark Comparison${NC}"
echo -e "${BOLD}${BLUE}          GoKit vs Chi vs Echo${NC}"
echo -e "${BOLD}${BLUE}==================================================================${NC}"
echo

# Install dependencies
echo -e "${YELLOW}Installing dependencies...${NC}"
go mod download

# Output file
RESULTS_FILE="benchmark_results_$(date +%Y%m%d_%H%M%S).txt"

# Run benchmarks with detailed output
echo -e "${GREEN}Running comprehensive benchmarks...${NC}"
echo -e "${GREEN}This may take a few minutes...${NC}"
echo

# Function to run and display benchmark
run_benchmark() {
    local pattern=$1
    local description=$2
    
    echo -e "${BOLD}${BLUE}$description${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    # Run benchmark and capture output
    output=$(go test -bench="$pattern" -benchmem -benchtime=10s -count=3 2>&1)
    
    # Display and save output
    echo "$output" | grep -E "Benchmark|ns/op|B/op|allocs/op" | while IFS= read -r line; do
        if [[ $line == *"GoKit"* ]]; then
            echo -e "${GREEN}$line${NC}"
        elif [[ $line == *"Chi"* ]]; then
            echo -e "${YELLOW}$line${NC}"
        elif [[ $line == *"Echo"* ]]; then
            echo -e "${BLUE}$line${NC}"
        else
            echo "$line"
        fi
    done
    
    # Save to file
    echo "$description" >> "$RESULTS_FILE"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" >> "$RESULTS_FILE"
    echo "$output" | grep -E "Benchmark|ns/op|B/op|allocs/op" >> "$RESULTS_FILE"
    echo >> "$RESULTS_FILE"
    
    echo
}

# Run different benchmark categories
run_benchmark "StaticRoute" "🚀 Static Route Performance"
run_benchmark "ParameterizedRoute" "🎯 Parameterized Route Performance"
run_benchmark "JSONResponse" "📦 JSON Response (Small Payload)"
run_benchmark "LargeJSON" "📊 Large JSON Response (1000 objects)"
run_benchmark "Middleware3" "⚙️  Middleware Chain (3 middlewares)"
run_benchmark "Middleware5" "⚙️  Middleware Chain (5 middlewares)"
run_benchmark "ParseJSON" "📝 JSON Request Parsing"
run_benchmark "ComplexRouting" "🗺️  Complex Routing (100 routes)"
run_benchmark "Parallel" "🔄 Concurrent Request Handling"

echo -e "${BOLD}${GREEN}==================================================================${NC}"
echo -e "${BOLD}${GREEN}                    BENCHMARK COMPLETE!${NC}"
echo -e "${BOLD}${GREEN}==================================================================${NC}"
echo
echo -e "${YELLOW}Full results saved to: ${BOLD}$RESULTS_FILE${NC}"
echo

# Generate summary comparison
echo -e "${BOLD}${BLUE}Quick Performance Summary:${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Extract and compare key metrics
echo -e "${BOLD}Static Route (ns/op - lower is better):${NC}"
grep -E "StaticRoute.*ns/op" "$RESULTS_FILE" | head -3 | while read -r line; do
    if [[ $line == *"GoKit"* ]]; then
        echo -e "  ${GREEN}GoKit: $(echo $line | awk '{print $3}')${NC}"
    elif [[ $line == *"Chi"* ]]; then
        echo -e "  ${YELLOW}Chi:   $(echo $line | awk '{print $3}')${NC}"
    elif [[ $line == *"Echo"* ]]; then
        echo -e "  ${BLUE}Echo:  $(echo $line | awk '{print $3}')${NC}"
    fi
done

echo
echo -e "${BOLD}Memory Allocations - Static Route (allocs/op - lower is better):${NC}"
grep -E "StaticRoute.*allocs/op" "$RESULTS_FILE" | head -3 | while read -r line; do
    if [[ $line == *"GoKit"* ]]; then
        echo -e "  ${GREEN}GoKit: $(echo $line | awk '{print $5}')${NC}"
    elif [[ $line == *"Chi"* ]]; then
        echo -e "  ${YELLOW}Chi:   $(echo $line | awk '{print $5}')${NC}"
    elif [[ $line == *"Echo"* ]]; then
        echo -e "  ${BLUE}Echo:  $(echo $line | awk '{print $5}')${NC}"
    fi
done

echo
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BOLD}Legend:${NC}"
echo -e "  ${GREEN}■${NC} GoKit"
echo -e "  ${YELLOW}■${NC} Chi"
echo -e "  ${BLUE}■${NC} Echo"
echo
echo -e "${BOLD}Metrics Explained:${NC}"
echo -e "  • ns/op:     Nanoseconds per operation (lower is better)"
echo -e "  • B/op:      Bytes allocated per operation (lower is better)"
echo -e "  • allocs/op: Number of allocations per operation (lower is better)"