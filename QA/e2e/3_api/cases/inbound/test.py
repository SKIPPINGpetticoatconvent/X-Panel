import json
import os
import sys
import time
import unittest

# Add parent directory to path to import common
sys.path.append(
    os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
)
from common.client import APIClient


class TestInbound(unittest.TestCase):
    def setUp(self):
        self.client = APIClient()
        if not self.client.login():
            self.skipTest("Login failed, skipping inbound tests")

    def test_add_and_get_inbound(self):
        # 1. Add Inbound
        # This is a basic VLESS+Vision template for testing
        remark = f"AutoTest_{int(time.time())}"
        port = 50000 + (int(time.time()) % 1000)  # Simple random port

        inbound_data = {
            "up": 0,
            "down": 0,
            "total": 0,
            "remark": remark,
            "enable": True,
            "expiryTime": 0,
            "listen": "",
            "port": port,
            "protocol": "vless",
            "settings": json.dumps(
                {
                    "clients": [
                        {
                            "id": "3b4f6b03-514d-4e9f-a2e6-123456789012",
                            "flow": "xtls-rprx-vision",
                            "email": f"test_{int(time.time())}@test.com",
                            "limitIp": 0,
                            "totalGB": 0,
                            "expiryTime": 0,
                            "enable": True,
                            "tgId": "",
                            "subId": "",
                        }
                    ],
                    "decryption": "none",
                    "fallbacks": [],
                }
            ),
            "streamSettings": json.dumps(
                {
                    "network": "tcp",
                    "security": "reality",
                    "realitySettings": {
                        "show": False,
                        "dest": "www.google.com:443",
                        "xver": 0,
                        "serverNames": ["www.google.com"],
                        "privateKey": "GenericPrivateKeyForTestingOnly",  # In real test this should be valid or generated
                        "shortIds": [""],
                    },
                    "tcpSettings": {"header": {"type": "none"}},
                }
            ),
            "sniffing": json.dumps({"enabled": True, "destOverride": ["http", "tls"]}),
        }

        resp = self.client.post("/panel/api/inbounds/add", data=inbound_data)

        # Note: If server validation strictness varies, this might fail e.g. on PrivateKey format
        # But we just want to test if the API endpoint responds.
        # If 200, it means success usually in this panel.

        if resp.status_code != 200:
            print(f"Add Inbound Failed: {resp.text}")

        self.assertTrue(resp.json()["success"], f"Failed to add inbound: {resp.text}")

        # 2. Verify in List
        list_resp = self.client.post("/panel/api/inbounds/list")
        self.assertTrue(list_resp.json()["success"])

        found = False
        for node in list_resp.json()["obj"]:
            if node["remark"] == remark:
                found = True
                break

        self.assertTrue(found, "Newly added inbound not found in list")


if __name__ == "__main__":
    unittest.main()
