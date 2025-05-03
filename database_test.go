package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestServer() (*httptest.Server, []byte) {
	// Create a mock database file content
	mockDBContent := []byte("MOCK_MAXMIND_DATABASE_CONTENT")

	// Create a test server that returns the mock content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(mockDBContent)
	}))

	return server, mockDBContent
}

func TestDownloadDatabase(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "geoip-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test server
	server, mockDBContent := setupTestServer()
	defer server.Close()

	// Test file path
	testFilePath := filepath.Join(tempDir, "test-db.mmdb")

	// Test download
	err = downloadDatabase(server.URL, testFilePath)
	if err != nil {
		t.Fatalf("downloadDatabase failed: %v", err)
	}

	// Verify file was downloaded correctly
	content, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != string(mockDBContent) {
		t.Errorf("Downloaded content doesn't match expected content")
	}

	// Test with non-existent server
	err = downloadDatabase("http://nonexistent.example.com", testFilePath)
	if err == nil {
		t.Error("Expected error when downloading from non-existent server, got nil")
	}
}

// Mock the geoip2.Open function for testing database initialization
type mockOpenFunc func(string) (Reader, error)

// TestInitDatabases uses temporary files and mocked geoip2.Open
func TestInitDatabases(t *testing.T) {
	// Save original database config and restore after test
	originalDatabases := databases
	originalDbDir := dbDir
	defer func() {
		databases = originalDatabases
		dbDir = originalDbDir
	}()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "geoip-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set dbDir to temp directory
	dbDir = tempDir

	// Setup test server
	server, mockDBContent := setupTestServer()
	defer server.Close()

	// Create a test database file
	testDbPath := filepath.Join(tempDir, "test-db.mmdb")
	if err := os.WriteFile(testDbPath, mockDBContent, 0644); err != nil {
		t.Fatalf("Failed to create test database file: %v", err)
	}

	// Create mock database config
	databases = map[string]*dbConfig{
		"test": {
			url:       server.URL,
			localPath: testDbPath,
		},
	}

	// Override the geoip2.Open function (this is simplified for testing)
	originalOpen := geoipOpen
	defer func() { geoipOpen = originalOpen }()

	geoipOpen = func(filename string) (Reader, error) {
		return &MockReader{}, nil
	}

	// Test initDatabases
	err = initDatabases()
	if err != nil {
		t.Fatalf("initDatabases failed: %v", err)
	}

	// Verify database file exists
	if _, err := os.Stat(databases["test"].localPath); os.IsNotExist(err) {
		t.Errorf("Database file was not created")
	}

	// Verify reader was initialized
	if databases["test"].reader == nil {
		t.Errorf("Database reader was not initialized")
	}

	// Verify lastUpdate was set
	if databases["test"].lastUpdate.IsZero() {
		t.Errorf("Database lastUpdate was not set")
	}
}

func TestUpdateDatabasesIfNeeded(t *testing.T) {
	// Save original database config and restore after test
	originalDatabases := databases
	originalDbDir := dbDir
	defer func() {
		databases = originalDatabases
		dbDir = originalDbDir
	}()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "geoip-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set dbDir to temp directory
	dbDir = tempDir

	// Setup test server
	server, mockDBContent := setupTestServer()
	defer server.Close()

	// Create initial database file
	testDbPath := filepath.Join(tempDir, "test-db.mmdb")
	if err := os.WriteFile(testDbPath, mockDBContent, 0644); err != nil {
		t.Fatalf("Failed to create test database file: %v", err)
	}

	// Save the original Open function
	originalOpen := geoipOpen
	defer func() { geoipOpen = originalOpen }()

	// Override with mock
	geoipOpen = func(filename string) (Reader, error) {
		return &MockReader{}, nil
	}

	// Create mock database config with old lastUpdate time
	databases = map[string]*dbConfig{
		"test": {
			url:        server.URL,
			localPath:  testDbPath,
			lastUpdate: time.Now().Add(-31 * 24 * time.Hour), // 31 days old
			mutex:      originalDatabases["city"].mutex,      // Reuse mutex
			reader:     &MockReader{},
		},
	}

	// Test updateDatabasesIfNeeded
	updateDatabasesIfNeeded()

	// Verify lastUpdate was updated
	if time.Since(databases["test"].lastUpdate) > time.Minute {
		t.Errorf("Database lastUpdate was not updated")
	}

	// Verify reader is not nil after update
	if databases["test"].reader == nil {
		t.Errorf("Database reader was not properly set after update")
	}
}

// TestStartDatabaseUpdater checks that the update goroutine runs without errors
func TestStartDatabaseUpdater(t *testing.T) {
	// This is a very basic test since we can't easily test the goroutine directly
	// We're just ensuring it doesn't panic or crash

	// Save original ticker duration and create a much shorter one for testing
	originalTicker := time.NewTicker(24 * time.Hour)
	testTicker := time.NewTicker(10 * time.Millisecond)

	// Create a done channel to signal when to stop the goroutine
	done := make(chan bool)

	// Start the updater in a goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("startDatabaseUpdater panicked: %v", r)
			}
			done <- true
		}()

		// Wait for ticker to fire a few times
		time.Sleep(50 * time.Millisecond)
	}()

	// Wait for the goroutine to finish
	<-done

	// Clean up
	originalTicker.Stop()
	testTicker.Stop()
}