#!/usr/bin/env python3

import requests
import json
import sys
from datetime import datetime

# Configuration
API_BASE_URL = "http://localhost:8080/api/v0"
AUTH_TOKEN = "change-me-in-production"
DEVICE = "Kragujevac-4948-10G.otk.rs"

# Headers for authenticated requests
headers = {
    "Authorization": f"Bearer {AUTH_TOKEN}",
    "Content-Type": "application/json"
}

def print_test_header(test_name):
    print(f"\n{'='*60}")
    print(f"Test: {test_name}")
    print(f"Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print('='*60)

def print_response(response, show_body=True):
    print(f"Status Code: {response.status_code}")
    print(f"Headers: {dict(response.headers)}")
    if show_body:
        try:
            body = response.json()
            print(f"Response Body: {json.dumps(body, indent=2)}")
        except:
            print(f"Response Body (raw): {response.text}")

def test_gen_get():
    """Test gen command with GET request"""
    print_test_header("Gen Command (GET)")

    response = requests.get(
        f"{API_BASE_URL}/gen",
        params={"filters": DEVICE},
        headers=headers
    )

    print_response(response)

    if response.status_code == 200:
        data = response.json()
        if data.get("success"):
            print("✓ Gen GET request successful")
        else:
            print(f"⨯ Gen GET request failed: {data.get('error')}")
    else:
        print(f"⨯ Request failed with status {response.status_code}")

def test_gen_post():
    """Test gen command with POST request"""
    print_test_header("Gen Command (POST)")

    payload = {
        "filters": [DEVICE]
    }

    response = requests.post(
        f"{API_BASE_URL}/gen",
        json=payload,
        headers=headers
    )

    print_response(response)

    if response.status_code == 200:
        data = response.json()
        if data.get("success"):
            print("✓ Gen POST request successful")
            if data.get("results") and DEVICE in data["results"]:
                result = data["results"][DEVICE]
                print(f"  Container: {result.get('container')}")
                print(f"  Exit Code: {result.get('exit_code')}")
                if result.get("stdout"):
                    print(f"  Config Output Length: {len(result['stdout'])} characters")
        else:
            print(f"⨯ Gen POST request failed: {data.get('error')}")
    else:
        print(f"⨯ Request failed with status {response.status_code}")

def test_gen_with_generators():
    """Test gen command with generator filters"""
    print_test_header("Gen Command with Generator Filters")

    payload = {
        "filters": [DEVICE],
        "generators": ["interfaces", "routing"]
    }

    response = requests.post(
        f"{API_BASE_URL}/gen",
        json=payload,
        headers=headers
    )

    print_response(response)

    if response.status_code == 200:
        data = response.json()
        if data.get("success"):
            print("✓ Gen with generators successful")
        else:
            print(f"⨯ Gen with generators failed: {data.get('error')}")
    else:
        print(f"⨯ Request failed with status {response.status_code}")

def test_diff():
    """Test diff command"""
    print_test_header("Diff Command")

    payload = {
        "filters": [DEVICE]
    }

    response = requests.post(
        f"{API_BASE_URL}/diff",
        json=payload,
        headers=headers
    )

    print_response(response)

    if response.status_code == 200:
        data = response.json()
        if data.get("success"):
            print("✓ Diff request successful")
            if data.get("results") and DEVICE in data["results"]:
                result = data["results"][DEVICE]
                print(f"  Container: {result.get('container')}")
                print(f"  Exit Code: {result.get('exit_code')}")
        else:
            print(f"⨯ Diff request failed: {data.get('error')}")
    else:
        print(f"⨯ Request failed with status {response.status_code}")

def test_patch_dry_run():
    """Test patch command with dry-run"""
    print_test_header("Patch Command (Dry Run)")

    payload = {
        "filters": [DEVICE],
        "dry_run": True
    }

    response = requests.post(
        f"{API_BASE_URL}/patch",
        json=payload,
        headers=headers
    )

    print_response(response)

    if response.status_code == 200:
        data = response.json()
        if data.get("success"):
            print("✓ Patch dry-run successful")
        else:
            print(f"⨯ Patch dry-run failed: {data.get('error')}")
    else:
        print(f"⨯ Request failed with status {response.status_code}")

def test_health():
    """Test health endpoint"""
    print_test_header("Health Check")

    response = requests.get(
        f"{API_BASE_URL}/health",
        headers=headers
    )

    print_response(response)

    if response.status_code == 200:
        print("✓ Health check passed")
    else:
        print(f"⨯ Health check failed with status {response.status_code}")

def test_containers():
    """Test container status endpoint"""
    print_test_header("Container Status")

    response = requests.get(
        f"{API_BASE_URL}/containers",
        headers=headers
    )

    print_response(response)

    if response.status_code == 200:
        data = response.json()
        print("✓ Container status retrieved")
        for container_name, status in data.items():
            print(f"  {container_name}:")
            print(f"    Running: {status.get('running')}")
            print(f"    Configured: {status.get('configured')}")
    else:
        print(f"⨯ Failed to get container status with status {response.status_code}")

def test_routing():
    """Test routing information for device"""
    print_test_header("Routing Information")

    response = requests.get(
        f"{API_BASE_URL}/routing",
        params={"hostname": DEVICE},
        headers=headers
    )

    print_response(response)

    if response.status_code == 200:
        data = response.json()
        print("✓ Routing information retrieved")
        if data.get("container"):
            print(f"  Device routed to container: {data['container']}")
        else:
            print("  No specific routing found (will use default container)")
    else:
        print(f"⨯ Failed to get routing with status {response.status_code}")

def test_parallel_execution():
    """Test parallel execution with multiple devices"""
    print_test_header("Parallel Execution")

    payload = {
        "filters": [DEVICE, f"{DEVICE}-backup"],
        "parallel": True,
        "timeout": 30
    }

    response = requests.post(
        f"{API_BASE_URL}/gen",
        json=payload,
        headers=headers
    )

    print_response(response)

    if response.status_code == 200:
        data = response.json()
        print(f"✓ Parallel execution completed")
        print(f"  Total hosts: {data.get('total_hosts')}")
        print(f"  Successful: {data.get('success_hosts')}")
        print(f"  Failed: {data.get('failed_hosts')}")
    else:
        print(f"⨯ Request failed with status {response.status_code}")

def test_quiet_mode():
    """Test quiet mode (suppress warnings)"""
    print_test_header("Quiet Mode")

    payload = {
        "filters": [DEVICE],
        "quiet": True
    }

    response = requests.post(
        f"{API_BASE_URL}/gen",
        json=payload,
        headers=headers
    )

    print_response(response)

    if response.status_code == 200:
        data = response.json()
        if data.get("success"):
            print("✓ Quiet mode execution successful")
            if data.get("results") and DEVICE in data["results"]:
                result = data["results"][DEVICE]
                if not result.get("stderr"):
                    print("  ✓ Stderr suppressed as expected")
        else:
            print(f"⨯ Quiet mode failed: {data.get('error')}")
    else:
        print(f"⨯ Request failed with status {response.status_code}")

def main():
    print(f"\nTesting annet-oil API")
    print(f"API URL: {API_BASE_URL}")
    print(f"Device: {DEVICE}")

    # Check if API is accessible
    try:
        response = requests.get(f"{API_BASE_URL}/health", headers=headers, timeout=5)
        if response.status_code != 200:
            print(f"\n⨯ API health check failed. Is the server running?")
            sys.exit(1)
    except requests.exceptions.ConnectionError:
        print(f"\n⨯ Cannot connect to API at {API_BASE_URL}")
        print("Please ensure the annet-oil server is running.")
        sys.exit(1)

    # Run tests
    tests = [
        test_health,
        test_containers,
        test_routing,
        test_gen_get,
        test_gen_post,
        test_gen_with_generators,
        test_diff,
        test_patch_dry_run,
        test_parallel_execution,
        test_quiet_mode
    ]

    for test in tests:
        try:
            test()
        except Exception as e:
            print(f"\n⨯ Test failed with exception: {e}")

    print("\n" + "="*60)
    print("Testing completed!")
    print("="*60)

if __name__ == "__main__":
    main()