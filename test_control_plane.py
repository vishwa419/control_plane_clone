#!/usr/bin/env python3
"""
Test script for Cloudflare Control Plane
Tests all endpoints: upload, consumer, and health checks
"""

import requests
import json
import os
import sys
import time
from pathlib import Path

# Configuration
UPLOAD_URL = "http://localhost:8080"
CONSUMER_URL = "http://localhost:8081"

# Test colors
GREEN = "\033[92m"
RED = "\033[91m"
YELLOW = "\033[93m"
BLUE = "\033[94m"
RESET = "\033[0m"


def print_test(name):
    """Print test name"""
    print(f"\n{BLUE}Testing: {name}{RESET}")


def print_success(message):
    """Print success message"""
    print(f"{GREEN}✓ {message}{RESET}")


def print_error(message):
    """Print error message"""
    print(f"{RED}✗ {message}{RESET}")


def print_info(message):
    """Print info message"""
    print(f"{YELLOW}  {message}{RESET}")


def test_health_checks():
    """Test health check endpoints"""
    print_test("Health Checks")
    
    # Test upload server health
    try:
        response = requests.get(f"{UPLOAD_URL}/health", timeout=5)
        if response.status_code == 200:
            print_success(f"Upload server health check: {response.json()}")
        else:
            print_error(f"Upload server health check failed: {response.status_code}")
            return False
    except Exception as e:
        print_error(f"Upload server health check failed: {e}")
        return False
    
    # Test consumer server health
    try:
        response = requests.get(f"{CONSUMER_URL}/health", timeout=5)
        if response.status_code == 200:
            print_success(f"Consumer server health check: {response.json()}")
        else:
            print_error(f"Consumer server health check failed: {response.status_code}")
            return False
    except Exception as e:
        print_error(f"Consumer server health check failed: {e}")
        return False
    
    return True


def test_file_upload(filename, version, content):
    """Test file upload"""
    print_test(f"File Upload: {filename} v{version}")
    
    # Create a temporary file
    test_file = Path(f"/tmp/test_{filename}_{version}.txt")
    test_file.write_text(content)
    
    try:
        with open(test_file, 'rb') as f:
            files = {'file': (filename, f, 'text/plain')}
            data = {
                'filename': filename,
                'version': version
            }
            response = requests.post(
                f"{UPLOAD_URL}/upload",
                files=files,
                data=data,
                timeout=10
            )
        
        if response.status_code == 200:
            result = response.json()
            if result.get('success'):
                metadata = result.get('metadata', {})
                print_success(f"File uploaded successfully")
                print_info(f"  Checksum: {metadata.get('checksum', 'N/A')[:16]}...")
                print_info(f"  Size: {metadata.get('size', 'N/A')} bytes")
                print_info(f"  Path: {metadata.get('filepath', 'N/A')}")
                return True, metadata
            else:
                print_error(f"Upload failed: {result.get('message', 'Unknown error')}")
                return False, None
        else:
            print_error(f"Upload failed with status {response.status_code}")
            print_info(f"  Response: {response.text}")
            return False, None
    except Exception as e:
        print_error(f"Upload exception: {e}")
        return False, None
    finally:
        # Clean up temp file
        if test_file.exists():
            test_file.unlink()


def test_get_latest_file(filename):
    """Test getting latest version of a file"""
    print_test(f"Get Latest File: {filename}")
    
    try:
        response = requests.get(f"{CONSUMER_URL}/file/{filename}", timeout=10)
        
        if response.status_code == 200:
            print_success(f"Retrieved latest version of {filename}")
            print_info(f"  Content length: {len(response.content)} bytes")
            print_info(f"  Version header: {response.headers.get('X-File-Version', 'N/A')}")
            print_info(f"  Checksum header: {response.headers.get('X-File-Checksum', 'N/A')[:16]}...")
            return True, response.content
        else:
            print_error(f"Failed to retrieve file: {response.status_code}")
            print_info(f"  Response: {response.text}")
            return False, None
    except Exception as e:
        print_error(f"Get file exception: {e}")
        return False, None


def test_get_file_version(filename, version):
    """Test getting specific version of a file"""
    print_test(f"Get File Version: {filename} v{version}")
    
    try:
        response = requests.get(f"{CONSUMER_URL}/file/{filename}/version/{version}", timeout=10)
        
        if response.status_code == 200:
            print_success(f"Retrieved {filename} version {version}")
            print_info(f"  Content length: {len(response.content)} bytes")
            print_info(f"  Version header: {response.headers.get('X-File-Version', 'N/A')}")
            return True, response.content
        else:
            print_error(f"Failed to retrieve file version: {response.status_code}")
            print_info(f"  Response: {response.text}")
            return False, None
    except Exception as e:
        print_error(f"Get file version exception: {e}")
        return False, None


def test_get_file_info(filename):
    """Test getting file metadata"""
    print_test(f"Get File Info: {filename}")
    
    try:
        response = requests.get(f"{CONSUMER_URL}/file/{filename}/info", timeout=10)
        
        if response.status_code == 200:
            info = response.json()
            print_success(f"Retrieved file info for {filename}")
            print_info(f"  Filename: {info.get('filename', 'N/A')}")
            print_info(f"  Version: {info.get('version', 'N/A')}")
            print_info(f"  Checksum: {info.get('checksum', 'N/A')[:16]}...")
            print_info(f"  Size: {info.get('size', 'N/A')} bytes")
            print_info(f"  Uploaded: {info.get('uploaded_at', 'N/A')}")
            return True, info
        else:
            print_error(f"Failed to retrieve file info: {response.status_code}")
            print_info(f"  Response: {response.text}")
            return False, None
    except Exception as e:
        print_error(f"Get file info exception: {e}")
        return False, None


def test_nonexistent_file():
    """Test handling of nonexistent file"""
    print_test("Nonexistent File Handling")
    
    try:
        response = requests.get(f"{CONSUMER_URL}/file/nonexistent_file_12345", timeout=5)
        
        if response.status_code == 404:
            print_success("Correctly returned 404 for nonexistent file")
            return True
        else:
            print_error(f"Expected 404, got {response.status_code}")
            return False
    except Exception as e:
        print_error(f"Exception: {e}")
        return False


def main():
    """Run all tests"""
    print(f"\n{BLUE}{'='*60}")
    print("Cloudflare Control Plane Test Suite")
    print(f"{'='*60}{RESET}\n")
    
    # Check if servers are reachable
    print_test("Server Connectivity")
    try:
        requests.get(f"{UPLOAD_URL}/health", timeout=2)
        requests.get(f"{CONSUMER_URL}/health", timeout=2)
        print_success("Both servers are reachable")
    except Exception as e:
        print_error(f"Servers not reachable: {e}")
        print_info("Make sure docker compose is running: docker compose up -d")
        sys.exit(1)
    
    results = []
    
    # Test 1: Health checks
    results.append(("Health Checks", test_health_checks()))
    
    # Test 2: Upload a file
    test_content_v1 = "Hello, World! This is version 1.0.0\n"
    success, metadata_v1 = test_file_upload("test-config", "1.0.0", test_content_v1)
    results.append(("File Upload v1.0.0", success))
    
    if not success:
        print_error("Cannot continue tests without successful upload")
        print_summary(results)
        sys.exit(1)
    
    # Wait a bit for Redis to sync
    time.sleep(0.5)
    
    # Test 3: Get file info
    success, info = test_get_file_info("test-config")
    results.append(("Get File Info", success))
    
    # Test 4: Get latest file
    success, content = test_get_latest_file("test-config")
    results.append(("Get Latest File", success))
    
    if success and content:
        # Verify content matches
        if content.decode('utf-8') == test_content_v1:
            print_success("Content matches uploaded file")
        else:
            print_error("Content mismatch!")
            print_info(f"  Expected: {test_content_v1[:50]}...")
            print_info(f"  Got: {content.decode('utf-8')[:50]}...")
    
    # Test 5: Upload a new version
    test_content_v2 = "Hello, World! This is version 2.0.0\nUpdated content here.\n"
    success, metadata_v2 = test_file_upload("test-config", "2.0.0", test_content_v2)
    results.append(("File Upload v2.0.0", success))
    
    if success:
        time.sleep(0.5)
        
        # Test 6: Get latest should return v2.0.0
        success, content = test_get_latest_file("test-config")
        results.append(("Get Latest File (should be v2.0.0)", success))
        
        if success and content:
            if content.decode('utf-8') == test_content_v2:
                print_success("Latest version correctly returns v2.0.0")
            else:
                print_error("Latest version content mismatch!")
        
        # Test 7: Get specific version v1.0.0
        success, content = test_get_file_version("test-config", "1.0.0")
        results.append(("Get Specific Version v1.0.0", success))
        
        if success and content:
            if content.decode('utf-8') == test_content_v1:
                print_success("Specific version content matches")
            else:
                print_error("Specific version content mismatch!")
        
        # Test 8: Get specific version v2.0.0
        success, content = test_get_file_version("test-config", "2.0.0")
        results.append(("Get Specific Version v2.0.0", success))
    
    # Test 9: Nonexistent file
    results.append(("Nonexistent File Handling", test_nonexistent_file()))
    
    # Print summary
    print_summary(results)


def print_summary(results):
    """Print test summary"""
    print(f"\n{BLUE}{'='*60}")
    print("Test Summary")
    print(f"{'='*60}{RESET}\n")
    
    passed = sum(1 for _, result in results if result)
    total = len(results)
    
    for test_name, result in results:
        status = f"{GREEN}PASS{RESET}" if result else f"{RED}FAIL{RESET}"
        print(f"  {status} - {test_name}")
    
    print(f"\n{BLUE}Total: {passed}/{total} tests passed{RESET}")
    
    if passed == total:
        print(f"{GREEN}All tests passed! ✓{RESET}\n")
    else:
        print(f"{RED}Some tests failed! ✗{RESET}\n")
        sys.exit(1)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print(f"\n{YELLOW}Tests interrupted by user{RESET}")
        sys.exit(1)
    except Exception as e:
        print_error(f"Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
