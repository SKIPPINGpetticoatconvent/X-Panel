import os
import sys
import unittest

import requests

# Add parent directory to path to import common
sys.path.append(
    os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
)
import common.config as config


class TestSecurity(unittest.TestCase):
    def test_stealth_api(self):
        # Re-implementation of test_stealth_api
        url = f"{config.BASE_URL}/panel/api/inbounds/list"
        print(f"Testing Stealth API on {url}...")
        try:
            resp = requests.get(url)
            # Expect 404 because we are not logged in / it is protected
            self.assertEqual(
                resp.status_code,
                404,
                "Stealth API should return 404 when not logged in",
            )
        except Exception as e:
            self.fail(f"Exception during request: {e}")


if __name__ == "__main__":
    unittest.main()
