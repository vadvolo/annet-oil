#!/bin/bash

# Configuration
API_URL="http://localhost:8080/api/v0"
AUTH_TOKEN="change-me-in-production"
DEVICE="Kragujevac-4948-10G.otk.rs"

echo "Testing annet-oil API with device: $DEVICE"
echo "=========================================="
echo ""

# Test 1: Gen command with GET request
echo "Test 1: Gen command (GET request)"
echo "----------------------------------"
curl -X GET \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  "$API_URL/gen?filters=$DEVICE" \
  -w "\nHTTP Status: %{http_code}\n" \
  | python3 -m json.tool

echo ""
echo ""

# Test 2: Gen command with POST request
echo "Test 2: Gen command (POST request)"
echo "----------------------------------"
curl -X POST \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"filters\": [\"$DEVICE\"]}" \
  "$API_URL/gen" \
  -w "\nHTTP Status: %{http_code}\n" \
  | python3 -m json.tool

echo ""
echo ""

# Test 3: Gen command with generator filter
echo "Test 3: Gen command with generator filter"
echo "------------------------------------------"
curl -X POST \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"filters\": [\"$DEVICE\"], \"generators\": [\"interfaces\"]}" \
  "$API_URL/gen" \
  -w "\nHTTP Status: %{http_code}\n" \
  | python3 -m json.tool

echo ""
echo ""

# Test 4: Diff command
echo "Test 4: Diff command"
echo "--------------------"
curl -X POST \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"filters\": [\"$DEVICE\"]}" \
  "$API_URL/diff" \
  -w "\nHTTP Status: %{http_code}\n" \
  | python3 -m json.tool

echo ""
echo ""

# Test 5: Health check
echo "Test 5: Health check"
echo "--------------------"
curl -X GET \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  "$API_URL/health" \
  -w "\nHTTP Status: %{http_code}\n"

echo ""
echo ""

# Test 6: Container status
echo "Test 6: Container status"
echo "------------------------"
curl -X GET \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  "$API_URL/containers" \
  -w "\nHTTP Status: %{http_code}\n" \
  | python3 -m json.tool

echo ""
echo ""

# Test 7: Routing information
echo "Test 7: Routing information for device"
echo "--------------------------------------"
curl -X GET \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  "$API_URL/routing?hostname=$DEVICE" \
  -w "\nHTTP Status: %{http_code}\n" \
  | python3 -m json.tool

echo ""
echo "Testing complete!"