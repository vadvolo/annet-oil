#!/usr/bin/env python3
"""Test script for filtered generator functionality in annet-oil API"""

import requests
import json
import os
from typing import Dict, List, Optional

# Configuration
API_URL = os.getenv("ANNET_OIL_API_URL", "http://localhost:8080")
AUTH_TOKEN = os.getenv("ANNET_OIL_AUTH_TOKEN", "change-me-in-production")

headers = {
    "Authorization": f"Bearer {AUTH_TOKEN}",
    "Content-Type": "application/json"
}


def test_command(command: str, filters: Optional[List[str]] = None,
                generators: Optional[List[str]] = None,
                exclude_generators: Optional[List[str]] = None) -> Dict:
    """Test a command with filtered generators"""

    payload = {
        "filters": filters or ["test-device"],
        "dry_run": True,
        "quiet": False
    }

    if generators:
        payload["generators"] = generators

    if exclude_generators:
        payload["exclude_generators"] = exclude_generators

    url = f"{API_URL}/api/v0/{command}"

    print(f"\n{'='*60}")
    print(f"Testing: {command}")
    print(f"Payload: {json.dumps(payload, indent=2)}")
    print(f"{'='*60}")

    try:
        response = requests.post(url, headers=headers, json=payload, timeout=30)
        response.raise_for_status()
        result = response.json()

        print(f"Success: {result.get('success')}")
        print(f"Total hosts: {result.get('total_hosts')}")
        print(f"Success hosts: {result.get('success_hosts')}")
        print(f"Failed hosts: {result.get('failed_hosts')}")

        if result.get("results"):
            for hostname, host_result in result["results"].items():
                print(f"\nHost: {hostname}")
                print(f"  Container: {host_result.get('container')}")
                print(f"  Exit Code: {host_result.get('exit_code')}")
                if host_result.get("stdout"):
                    print(f"  Output: {host_result['stdout'][:200]}...")
                if host_result.get("stderr"):
                    print(f"  Stderr: {host_result['stderr'][:200]}...")

        return result

    except requests.exceptions.RequestException as e:
        print(f"Error: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"Response: {e.response.text}")
        return {}


def main():
    """Run test scenarios"""

    print("Testing Annet Oil API with filtered generators")
    print(f"API URL: {API_URL}")

    # Test 1: Gen with specific generator
    print("\n\nTest 1: Generate with specific generator (description)")
    test_command("gen",
                filters=["router1.example.com"],
                generators=["description"])

    # Test 2: Gen with exclude generator
    print("\n\nTest 2: Generate excluding specific generator (hostname)")
    test_command("gen",
                filters=["router1.example.com"],
                exclude_generators=["hostname"])

    # Test 3: Gen with both include and exclude
    print("\n\nTest 3: Generate with both include and exclude generators")
    test_command("gen",
                filters=["router1.example.com"],
                generators=["interfaces", "routing"],
                exclude_generators=["acl"])

    # Test 4: Diff with generator filter
    print("\n\nTest 4: Diff with generator filter")
    test_command("diff",
                filters=["switch1.example.com"],
                generators=["vlans"])

    # Test 5: Multiple devices with generator filter
    print("\n\nTest 5: Multiple devices with generator filter")
    test_command("gen",
                filters=["router1.example.com", "router2.example.com"],
                generators=["interfaces"])


if __name__ == "__main__":
    main()