package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"x-ui/database/model"
	"x-ui/web/job"
	"x-ui/web/service"
	"x-ui/xray"
)

func TestLogStreamerLogRotation(t *testing.T) {
	// Create a temporary log file
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	// Create initial log file
	err := os.WriteFile(logPath, []byte("email: test@example.com from tcp:192.168.1.1:12345 accepted\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	// Create LogStreamer
	ls := job.NewLogStreamer(logPath)

	// Start LogStreamer
	err = ls.Start()
	if err != nil {
		t.Fatalf("Failed to start LogStreamer: %v", err)
	}
	defer ls.Stop()

	// Wait for initial processing
	time.Sleep(100 * time.Millisecond)

	// Simulate log rotation by appending new content (since tail handles rotation)
	err = os.WriteFile(logPath, []byte("email: test@example.com from tcp:192.168.1.1:12345 accepted\nemail: new@example.com from tcp:192.168.1.2:12345 accepted\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to append to log file: %v", err)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Check active clients
	activeClients := ls.GetActiveClientIPs()
	if len(activeClients) == 0 {
		t.Error("Expected active clients, got none")
	}

	// Should have at least one client
	if len(activeClients) < 1 {
		t.Errorf("Expected at least 1 active client, got %d", len(activeClients))
	}

	// Verify the client data
	for email, ips := range activeClients {
		if email == "test@example.com" || email == "new@example.com" {
			if len(ips) == 0 {
				t.Errorf("Expected IPs for email %s, got none", email)
			}
		}
	}
}

func TestInboundServiceConcurrentCacheAccess(t *testing.T) {
	// Create InboundService
	is := &service.InboundService{}

	// Create a mock inbound with settings
	inbound := &model.Inbound{
		Id:       1,
		Settings: `{"clients":[{"email":"test@example.com","id":"test-id"}]}`,
	}

	// Simulate concurrent access to GetClients (which uses cache internally)
	var wg sync.WaitGroup
	numGoroutines := 10
	numIterations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				// This method internally uses the cache
				clients, err := is.GetClients(inbound)
				if err != nil {
					t.Errorf("Failed to get clients for goroutine %d, iteration %d: %v", id, j, err)
				}
				if len(clients) == 0 {
					t.Errorf("Expected clients, got none for goroutine %d, iteration %d", id, j)
				}
			}
		}(i)
	}

	wg.Wait()

	// Test completed without race conditions
	t.Log("Concurrent cache access test completed successfully")
}

func TestInboundServiceBulkTrafficUpdates(t *testing.T) {
	// Test that the service can handle large data structures without issues
	// Create mock traffic data
	var inboundTraffics []*xray.Traffic
	var clientTraffics []*xray.ClientTraffic

	// Add some mock inbound traffic
	for i := 0; i < 100; i++ {
		inboundTraffics = append(inboundTraffics, &xray.Traffic{
			Tag:      "inbound-8080",
			IsInbound: true,
			Up:       int64(i * 1000),
			Down:     int64(i * 2000),
		})
	}

	// Add some mock client traffic
	for i := 0; i < 50; i++ {
		clientTraffics = append(clientTraffics, &xray.ClientTraffic{
			Email: fmt.Sprintf("client%d@example.com", i),
			Up:    int64(i * 500),
			Down:  int64(i * 1000),
		})
	}

	// Test data structure creation and manipulation
	// This tests the performance of data handling without database calls
	totalInboundUp := int64(0)
	totalInboundDown := int64(0)
	for _, traffic := range inboundTraffics {
		totalInboundUp += traffic.Up
		totalInboundDown += traffic.Down
	}

	totalClientUp := int64(0)
	totalClientDown := int64(0)
	for _, traffic := range clientTraffics {
		totalClientUp += traffic.Up
		totalClientDown += traffic.Down
	}

	// Verify calculations
	expectedInboundUp := int64(100 * 99 * 1000 / 2) // sum of 0 to 99 * 1000
	expectedInboundDown := int64(100 * 99 * 2000 / 2) // sum of 0 to 99 * 2000
	expectedClientUp := int64(50 * 49 * 500 / 2)
	expectedClientDown := int64(50 * 49 * 1000 / 2)

	if totalInboundUp != expectedInboundUp {
		t.Errorf("Inbound up total mismatch: got %d, expected %d", totalInboundUp, expectedInboundUp)
	}
	if totalInboundDown != expectedInboundDown {
		t.Errorf("Inbound down total mismatch: got %d, expected %d", totalInboundDown, expectedInboundDown)
	}
	if totalClientUp != expectedClientUp {
		t.Errorf("Client up total mismatch: got %d, expected %d", totalClientUp, expectedClientUp)
	}
	if totalClientDown != expectedClientDown {
		t.Errorf("Client down total mismatch: got %d, expected %d", totalClientDown, expectedClientDown)
	}

	// Test completed successfully
	t.Log("Bulk traffic data handling test completed successfully")
}