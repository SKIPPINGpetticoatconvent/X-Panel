package scenarios

import (
	"testing"

	"x-ui/tests/e2e/api"
)

func TestAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	baseURL := GetTestBaseURL()
	username, password := GetTestCredentials()

	// 测试成功登录
	t.Run("SuccessfulLogin", func(t *testing.T) {
		client, err := api.NewClient(baseURL)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		err = client.Login(username, password)
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		t.Log("Login successful")
	})

	// 测试无效凭据登录
	t.Run("InvalidCredentials", func(t *testing.T) {
		client, err := api.NewClient(baseURL)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		err = client.Login("invalid", "invalid")
		if err == nil {
			t.Error("Expected login to fail with invalid credentials")
		} else {
			t.Logf("Invalid login correctly failed: %v", err)
		}
	})

	// 测试空凭据登录
	t.Run("EmptyCredentials", func(t *testing.T) {
		client, err := api.NewClient(baseURL)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		err = client.Login("", "")
		if err == nil {
			t.Error("Expected login to fail with empty credentials")
		} else {
			t.Logf("Empty login correctly failed: %v", err)
		}
	})
}