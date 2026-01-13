import requests
import time

BASE_URL = "http://127.0.0.1:13688"

def test_stealth_api():
    print(f"Testing Stealth API on {BASE_URL}/panel/api/inbounds/list...")
    try:
        resp = requests.get(f"{BASE_URL}/panel/api/inbounds/list")
        if resp.status_code == 404:
            print("PASS: Stealth API returned 404 Not Found")
        else:
            print(f"FAIL: Stealth API returned {resp.status_code}")
    except Exception as e:
        print(f"FAIL: Exception {e}")

def test_login_limit():
    print("Testing Login Rate Limiting...")
    url = f"{BASE_URL}/login"
    
    # Send 5 failed attempts
    for i in range(5):
        try:
            resp = requests.post(url, data={"username": "admin", "password": f"wrong{i}"})
            print(f"Attempt {i+1}: Status {resp.status_code}, Response: {resp.text[:100]}...")
        except Exception as e:
            print(f"Attempt {i+1} FAIL: {e}")
            
    # Send 6th attempt (Should be blocked)
    print("Sending 6th attempt (expect block)...")
    try:
        resp = requests.post(url, data={"username": "admin", "password": "wrong6"})
        print(f"Attempt 6: Status {resp.status_code}, Response: {resp.text}")
        if "tooManyAttempts" in resp.text:
             print("PASS: Request was blocked with 'Too many attempts' message")
        # Trying to match translation key or response if i18n is default
        elif "尝试次数过多" in resp.text or "Too many attempts" in resp.text:
             print("PASS: Request was blocked with 'Too many attempts' message")
        else:
             print("FAIL: Request was NOT blocked properly")
    except Exception as e:
        print(f"Attempt 6 FAIL: {e}")

if __name__ == "__main__":
    # Wait for server to be fully ready
    time.sleep(2)
    test_stealth_api()
    test_login_limit()
