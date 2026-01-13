import os
import sys

import requests

# Add parent directory to path to import config
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
import common.config as config


class APIClient:
    def __init__(self):
        self.session = requests.Session()
        self.base_url = config.BASE_URL

    def login(self, username=config.DEFAULT_USERNAME, password=config.DEFAULT_PASSWORD):
        url = f"{self.base_url}/login"
        payload = {"username": username, "password": password}
        try:
            resp = self.session.post(url, data=payload)
            if resp.status_code == 200 and "success" in resp.text:
                return True
            return False
        except Exception as e:
            print(f"Login Exception: {e}")
            return False

    def get(self, endpoint):
        return self.session.get(f"{self.base_url}{endpoint}")

    def post(self, endpoint, data=None, json=None):
        return self.session.post(f"{self.base_url}{endpoint}", data=data, json=json)
