#!/bin/bash

# Complete test script: starts servers, runs tests, cleans up
# Usage: ./run_all.sh

# DON'T use set -e because we want to count failures, not exit on first failure

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    
    # Kill all background processes
    if [ ! -z "$PRIMARY_PID" ]; then
        echo "Stopping primary (PID: $PRIMARY_PID)"
        kill $PRIMARY_PID 2>/dev/null || true
    fi
    
    if [ ! -z "$BACKUP1_PID" ]; then
        echo "Stopping backup1 (PID: $BACKUP1_PID)"
        kill $BACKUP1_PID 2>/dev/null || true
    fi
    
    if [ ! -z "$BACKUP2_PID" ]; then
        echo "Stopping backup2 (PID: $BACKUP2_PID)"
        kill $BACKUP2_PID 2>/dev/null || true
    fi
    
    # Wait a bit for processes to terminate
    sleep 1
    
    # Force kill if still running
    pkill -9 -f "go run.*--primary" 2>/dev/null || true
    pkill -9 -f "go run.*--http=808" 2>/dev/null || true
    
    # Kill by port
    lsof -ti:8080 | xargs kill -9 2>/dev/null || true
    lsof -ti:8081 | xargs kill -9 2>/dev/null || true
    lsof -ti:8082 | xargs kill -9 2>/dev/null || true
    lsof -ti:8083 | xargs kill -9 2>/dev/null || true
    lsof -ti:8084 | xargs kill -9 2>/dev/null || true
    lsof -ti:8085 | xargs kill -9 2>/dev/null || true
    
    echo -e "${GREEN}Cleanup complete${NC}"
}

# Set trap to cleanup on exit
trap cleanup EXIT INT TERM

echo -e "${BLUE}=========================================="
echo "Replicated Key-Value Store Test Runner"
echo -e "==========================================${NC}\n"

# Check if go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    exit 1
fi

# Check if curl is installed
if ! command -v curl &> /dev/null; then
    echo -e "${RED}Error: curl is not installed${NC}"
    exit 1
fi

echo -e "${YELLOW}Step 1: Cleaning up any existing processes${NC}"
# More aggressive cleanup
pkill -9 -f "go run.*--primary" 2>/dev/null || true
pkill -9 -f "go run.*--http=808" 2>/dev/null || true
lsof -ti:8080 | xargs kill -9 2>/dev/null || true
lsof -ti:8081 | xargs kill -9 2>/dev/null || true
lsof -ti:8082 | xargs kill -9 2>/dev/null || true
lsof -ti:8083 | xargs kill -9 2>/dev/null || true
lsof -ti:8084 | xargs kill -9 2>/dev/null || true
lsof -ti:8085 | xargs kill -9 2>/dev/null || true
sleep 3
echo "Cleanup complete"

echo -e "${YELLOW}Step 2: Starting Primary server${NC}"
go run . --primary=true --port=8080 --http=8081 > primary.log 2>&1 &
PRIMARY_PID=$!
echo "Primary started (PID: $PRIMARY_PID)"
sleep 2

# Check if primary is running
if ! kill -0 $PRIMARY_PID 2>/dev/null; then
    echo -e "${RED}Error: Primary failed to start. Check primary.log${NC}"
    cat primary.log
    exit 1
fi

echo -e "${YELLOW}Step 3: Starting Backup1 server${NC}"
# Use expect to auto-answer the prompt, or use a workaround
(echo "127.0.0.1:8080" | go run . --primary=false --port=8082 --http=8083) > backup1.log 2>&1 &
BACKUP1_PID=$!
echo "Backup1 started (PID: $BACKUP1_PID)"
sleep 2

echo -e "${YELLOW}Step 4: Starting Backup2 server${NC}"
(echo "127.0.0.1:8080" | go run . --primary=false --port=8084 --http=8085) > backup2.log 2>&1 &
BACKUP2_PID=$!
echo "Backup2 started (PID: $BACKUP2_PID)"
sleep 3

# Verify all servers are running
if ! kill -0 $PRIMARY_PID 2>/dev/null; then
    echo -e "${RED}Error: Primary is not running${NC}"
    exit 1
fi

if ! kill -0 $BACKUP1_PID 2>/dev/null; then
    echo -e "${RED}Error: Backup1 is not running${NC}"
    exit 1
fi

if ! kill -0 $BACKUP2_PID 2>/dev/null; then
    echo -e "${RED}Error: Backup2 is not running${NC}"
    exit 1
fi

echo -e "${GREEN}All servers started successfully${NC}\n"

# Wait for servers to be ready
echo -e "${YELLOW}Step 5: Waiting for servers to be ready${NC}"
sleep 5  # Simple wait instead of health check

echo -e "${GREEN}Servers should be ready now${NC}\n"

echo -e "\n${YELLOW}Step 6: Running tests${NC}\n"

# Configuration
PRIMARY="http://localhost:8081"
BACKUP1="http://localhost:8083"
BACKUP2="http://localhost:8085"

# Test counter
PASS=0
FAIL=0

# Helper function to run a test
run_test() {
    local test_name=$1
    local command=$2
    local expected=$3
    
    echo -e "${BLUE}TEST: ${test_name}${NC}"
    
    result=$(eval $command 2>&1)
    
    if [[ $result == *"$expected"* ]]; then
        echo -e "${GREEN}✓ PASS${NC}\n"
        ((PASS++))
    else
        echo -e "${RED}✗ FAIL${NC}"
        echo "Expected: $expected"
        echo "Got: $result"
        echo ""
        ((FAIL++))
    fi
}

echo "=========================================="
echo "1. BASIC WRITE TESTS"
echo "=========================================="
echo ""

run_test "Write key1=value1" \
    "curl -s -X POST $PRIMARY/key1/value1" \
    '"key":"key1","value":"value1"'

run_test "Write key2=value2" \
    "curl -s -X POST $PRIMARY/key2/value2" \
    '"key":"key2","value":"value2"'

run_test "Write key3=value3" \
    "curl -s -X POST $PRIMARY/key3/value3" \
    '"key":"key3","value":"value3"'

echo "=========================================="
echo "2. BASIC READ TESTS"
echo "=========================================="
echo ""

run_test "Read key1 from primary" \
    "curl -s -X GET $PRIMARY/key1" \
    '"key":"key1","value":"value1"'

run_test "Read key2 from primary" \
    "curl -s -X GET $PRIMARY/key2" \
    '"key":"key2","value":"value2"'

run_test "Read non-existent key" \
    "curl -s -X GET $PRIMARY/nonexistent" \
    '"error":"Key not found"'

echo "=========================================="
echo "3. REPLICATION TESTS"
echo "=========================================="
echo ""

sleep 2  # Wait for replication

run_test "Read key1 from Backup1" \
    "curl -s -X GET $BACKUP1/key1" \
    '"key":"key1","value":"value1"'

run_test "Read key2 from Backup1" \
    "curl -s -X GET $BACKUP1/key2" \
    '"key":"key2","value":"value2"'

run_test "Read key1 from Backup2" \
    "curl -s -X GET $BACKUP2/key1" \
    '"key":"key1","value":"value1"'

run_test "Read key3 from Backup2" \
    "curl -s -X GET $BACKUP2/key3" \
    '"key":"key3","value":"value3"'

echo "=========================================="
echo "4. ERROR HANDLING"
echo "=========================================="
echo ""

run_test "Write to Backup1 (should fail)" \
    "curl -s -X POST $BACKUP1/test/fail" \
    '"error":"Only primary can accept writes"'

run_test "Write to Backup2 (should fail)" \
    "curl -s -X POST $BACKUP2/test/fail" \
    '"error":"Only primary can accept writes"'

run_test "Read non-existent from Backup1" \
    "curl -s -X GET $BACKUP1/doesnotexist" \
    '"error":"Key not found"'

echo "=========================================="
echo "5. UPDATE TESTS"
echo "=========================================="
echo ""

run_test "Update key1 to newvalue1" \
    "curl -s -X POST $PRIMARY/key1/newvalue1" \
    '"key":"key1","value":"newvalue1"'

sleep 2

run_test "Verify update on Primary" \
    "curl -s -X GET $PRIMARY/key1" \
    '"value":"newvalue1"'

run_test "Verify update on Backup1" \
    "curl -s -X GET $BACKUP1/key1" \
    '"value":"newvalue1"'

run_test "Verify update on Backup2" \
    "curl -s -X GET $BACKUP2/key1" \
    '"value":"newvalue1"'

echo "=========================================="
echo "6. SEQUENTIAL WRITES (LSN Ordering)"
echo "=========================================="
echo ""

for i in {1..5}; do
    curl -s -X POST $PRIMARY/seq$i/value$i > /dev/null
done

sleep 2

run_test "Verify seq1 on Backup1" \
    "curl -s -X GET $BACKUP1/seq1" \
    '"value":"value1"'

run_test "Verify seq3 on Backup2" \
    "curl -s -X GET $BACKUP2/seq3" \
    '"value":"value3"'

run_test "Verify seq5 on Backup1" \
    "curl -s -X GET $BACKUP1/seq5" \
    '"value":"value5"'

echo "=========================================="
echo "7. CONSISTENCY CHECK"
echo "=========================================="
echo ""

curl -s -X POST $PRIMARY/consistency/final > /dev/null
sleep 2

PRIMARY_VAL=$(curl -s -X GET $PRIMARY/consistency 2>/dev/null | grep -o '"value":"[^"]*"' | cut -d'"' -f4)
BACKUP1_VAL=$(curl -s -X GET $BACKUP1/consistency 2>/dev/null | grep -o '"value":"[^"]*"' | cut -d'"' -f4)
BACKUP2_VAL=$(curl -s -X GET $BACKUP2/consistency 2>/dev/null | grep -o '"value":"[^"]*"' | cut -d'"' -f4)

echo "Primary value: $PRIMARY_VAL"
echo "Backup1 value: $BACKUP1_VAL"
echo "Backup2 value: $BACKUP2_VAL"

if [ "$PRIMARY_VAL" == "$BACKUP1_VAL" ] && [ "$PRIMARY_VAL" == "$BACKUP2_VAL" ] && [ ! -z "$PRIMARY_VAL" ]; then
    echo -e "${GREEN}✓ All nodes consistent${NC}"
    ((PASS++))
else
    echo -e "${RED}✗ Consistency violation${NC}"
    ((FAIL++))
fi

echo ""
echo "=========================================="
echo "TEST SUMMARY"
echo "=========================================="
echo -e "${GREEN}Passed: $PASS${NC}"
echo -e "${RED}Failed: $FAIL${NC}"
echo "Total: $((PASS + FAIL))"
echo ""

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    EXIT_CODE=0
else
    echo -e "${RED}Some tests failed${NC}"
    echo -e "\nCheck logs for details:"
    echo "  - primary.log"
    echo "  - backup1.log"
    echo "  - backup2.log"
    EXIT_CODE=1
fi

echo ""
echo -e "${YELLOW}Logs saved to:${NC}"
echo "  - primary.log"
echo "  - backup1.log"
echo "  - backup2.log"

exit $EXIT_CODE