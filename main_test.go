package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpConfig := `{"host": "localhost", "port": "8080"}`
	tmpFile, err := os.CreateTemp("", "config*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(tmpConfig)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading the config
	err = loadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify the config was loaded correctly
	if config.Host != "localhost" {
		t.Errorf("Expected host to be 'localhost', got '%s'", config.Host)
	}
	if config.Port != "8080" {
		t.Errorf("Expected port to be '8080', got '%s'", config.Port)
	}

	// Test loading a non-existent file
	err = loadConfig("nonexistent.json")
	if err == nil {
		t.Error("Expected error when loading non-existent file, got nil")
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		request  *http.Request
		expected string
	}{
		{
			name:     "X-Forwarded-For header",
			request:  httptest.NewRequest(http.MethodGet, "/", nil),
			expected: "192.168.1.1",
		},
		{
			name:     "Multiple IPs in X-Forwarded-For",
			request:  httptest.NewRequest(http.MethodGet, "/", nil),
			expected: "192.168.1.1",
		},
		{
			name:     "Remote address only",
			request:  httptest.NewRequest(http.MethodGet, "/", nil),
			expected: "192.0.2.1",
		},
	}

	// Set up X-Forwarded-For headers
	tests[0].request.Header.Set("X-Forwarded-For", "192.168.1.1")
	tests[1].request.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1, 172.16.0.1")
	tests[2].request.RemoteAddr = "192.0.2.1:12345"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := getClientIP(tc.request)
			if ip != tc.expected {
				t.Errorf("Expected IP '%s', got '%s'", tc.expected, ip)
			}
		})
	}
}

func TestGetIPInfo(t *testing.T) {
	// Save original databases and restore after test
	originalDatabases := databases
	defer func() { databases = originalDatabases }()

	// Setup mock databases
	mockReader := &MockReader{}
	databases = map[string]*dbConfig{
		"asn": {
			reader: mockReader,
			mutex:  originalDatabases["asn"].mutex,
		},
		"city": {
			reader: mockReader,
			mutex:  originalDatabases["city"].mutex,
		},
		"country": {
			reader: mockReader,
			mutex:  originalDatabases["country"].mutex,
		},
	}

	// Test with IPv4
	ip := net.ParseIP("192.168.1.1")
	info, err := getIPInfo(ip)
	if err != nil {
		t.Fatalf("Failed to get IP info: %v", err)
	}

	// Verify IP info
	if info.IP != "192.168.1.1" {
		t.Errorf("Expected IP '192.168.1.1', got '%s'", info.IP)
	}
	if info.Version != "IPv4" {
		t.Errorf("Expected Version 'IPv4', got '%s'", info.Version)
	}
	if info.City != "Test City" {
		t.Errorf("Expected City 'Test City', got '%s'", info.City)
	}
	if info.Country != "TS" {
		t.Errorf("Expected Country 'TS', got '%s'", info.Country)
	}
	if info.ASN != "AS12345" {
		t.Errorf("Expected ASN 'AS12345', got '%s'", info.ASN)
	}

	// Test with IPv6
	ip = net.ParseIP("2001:db8::1")
	info, err = getIPInfo(ip)
	if err != nil {
		t.Fatalf("Failed to get IPv6 info: %v", err)
	}

	if info.Version != "IPv6" {
		t.Errorf("Expected Version 'IPv6', got '%s'", info.Version)
	}
}

func TestHandleIPLookup(t *testing.T) {
	// Save original databases and restore after test
	originalDatabases := databases
	defer func() { databases = originalDatabases }()

	// Setup mock databases
	mockReader := &MockReader{}
	databases = map[string]*dbConfig{
		"asn": {
			reader: mockReader,
			mutex:  originalDatabases["asn"].mutex,
		},
		"city": {
			reader: mockReader,
			mutex:  originalDatabases["city"].mutex,
		},
		"country": {
			reader: mockReader,
			mutex:  originalDatabases["country"].mutex,
		},
	}

	// Test valid IP
	req := httptest.NewRequest(http.MethodGet, "/ipgeo/192.168.1.1", nil)
	w := httptest.NewRecorder()

	handleIPLookup(w, req, "192.168.1.1")

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	var ipInfo IPInfo
	body, _ := io.ReadAll(resp.Body)
	err := json.Unmarshal(body, &ipInfo)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if ipInfo.IP != "192.168.1.1" {
		t.Errorf("Expected IP '192.168.1.1', got '%s'", ipInfo.IP)
	}

	// Test invalid IP
	req = httptest.NewRequest(http.MethodGet, "/ipgeo/invalid-ip", nil)
	w = httptest.NewRecorder()

	handleIPLookup(w, req, "invalid-ip")

	resp = w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status Bad Request for invalid IP, got %v", resp.Status)
	}
}

func TestHandleRequest(t *testing.T) {
	// Save original config and restore after test
	originalConfig := config
	defer func() { config = originalConfig }()

	// Save original databases and restore after test
	originalDatabases := databases
	defer func() { databases = originalDatabases }()

	// Setup mock databases
	mockReader := &MockReader{}
	databases = map[string]*dbConfig{
		"asn": {
			reader: mockReader,
			mutex:  originalDatabases["asn"].mutex,
		},
		"city": {
			reader: mockReader,
			mutex:  originalDatabases["city"].mutex,
		},
		"country": {
			reader: mockReader,
			mutex:  originalDatabases["country"].mutex,
		},
	}

	// Test with no host restriction
	config.Host = ""

	// Test /ipgeo endpoint
	req := httptest.NewRequest(http.MethodGet, "/ipgeo", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handleRequest(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK for /ipgeo, got %v", resp.Status)
	}

	// Test /ipgeo/{ip} endpoint
	req = httptest.NewRequest(http.MethodGet, "/ipgeo/8.8.8.8", nil)
	w = httptest.NewRecorder()

	handleRequest(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK for /ipgeo/{ip}, got %v", resp.Status)
	}

	// Test invalid endpoint
	req = httptest.NewRequest(http.MethodGet, "/invalid", nil)
	w = httptest.NewRecorder()

	handleRequest(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status Forbidden for invalid endpoint, got %v", resp.Status)
	}

	// Test with host restriction
	config.Host = "api.example.com"

	// Test with correct host
	req = httptest.NewRequest(http.MethodGet, "/ipgeo", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Host = "api.example.com"
	w = httptest.NewRecorder()

	handleRequest(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK for correct host, got %v", resp.Status)
	}

	// Test with incorrect host
	req = httptest.NewRequest(http.MethodGet, "/ipgeo", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Host = "wrong.example.com"
	w = httptest.NewRecorder()

	handleRequest(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status Forbidden for incorrect host, got %v", resp.Status)
	}
}