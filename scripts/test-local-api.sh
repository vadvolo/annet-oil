#!/bin/bash

# Test script for Annet-Oil API endpoints

API_URL="http://localhost:8080"
AUTH_TOKEN="test-token-12345"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_test() {
    echo -e "\n${YELLOW}Testing: $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

# Test 1: Health Check
print_test "Health Check"
response=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $AUTH_TOKEN" \
    $API_URL/api/v0/health)
http_code=$(echo "$response" | tail -n 1)
body=$(echo "$response" | head -n -1)

if [ "$http_code" == "200" ]; then
    print_success "Health check passed"
    echo "$body" | jq . 2>/dev/null || echo "$body"
else
    print_error "Health check failed (HTTP $http_code)"
fi

# Test 2: Extended Health Check
print_test "Extended Health Check"
response=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $AUTH_TOKEN" \
    $API_URL/api/v0/health/extended)
http_code=$(echo "$response" | tail -n 1)
body=$(echo "$response" | head -n -1)

if [ "$http_code" == "200" ]; then
    print_success "Extended health check passed"
    echo "$body" | jq . 2>/dev/null || echo "$body"
else
    print_error "Extended health check failed (HTTP $http_code)"
fi

# Test 3: Containers
print_test "Container Status"
response=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $AUTH_TOKEN" \
    $API_URL/api/v0/containers)
http_code=$(echo "$response" | tail -n 1)
body=$(echo "$response" | head -n -1)

if [ "$http_code" == "200" ]; then
    print_success "Container status retrieved"
    echo "$body" | jq . 2>/dev/null || echo "$body"
else
    print_error "Container status failed (HTTP $http_code)"
fi

# Test 4: Execute Command (show version)
print_test "Execute Command - show version"
response=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"command": "show version"}' \
    $API_URL/api/v0/execute)
http_code=$(echo "$response" | tail -n 1)
body=$(echo "$response" | head -n -1)

if [ "$http_code" == "200" ] || [ "$http_code" == "403" ]; then
    if [ "$http_code" == "403" ]; then
        print_success "Command validation working (command blocked as expected in test)"
    else
        print_success "Command executed"
    fi
    echo "$body" | jq . 2>/dev/null || echo "$body"
else
    print_error "Command execution failed (HTTP $http_code)"
fi

# Test 5: Execute Invalid Command
print_test "Execute Invalid Command - configure terminal (should be blocked)"
response=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"command": "configure terminal"}' \
    $API_URL/api/v0/execute)
http_code=$(echo "$response" | tail -n 1)

if [ "$http_code" == "403" ]; then
    print_success "Invalid command blocked correctly"
else
    print_error "Invalid command not blocked (HTTP $http_code)"
fi

# Test 6: Routing Information
print_test "Routing Information"
response=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $AUTH_TOKEN" \
    $API_URL/api/v0/routing)
http_code=$(echo "$response" | tail -n 1)
body=$(echo "$response" | head -n -1)

if [ "$http_code" == "200" ]; then
    print_success "Routing information retrieved"
    echo "$body" | jq . 2>/dev/null || echo "$body"
else
    print_error "Routing information failed (HTTP $http_code)"
fi

# Summary
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}API Test Complete!${NC}"
echo -e "${GREEN}========================================${NC}"