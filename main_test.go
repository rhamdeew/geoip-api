package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/oschwald/geoip2-golang"
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

func TestEnsureConfigFileExists(t *testing.T) {
	// Test case 1: Config file doesn't exist, should create with default values
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Path to a non-existent config file
	configPath := tempDir + "/config.json"

	// Ensure config file exists (should create it)
	err = ensureConfigFileExists(configPath)
	if err != nil {
		t.Fatalf("ensureConfigFileExists failed: %v", err)
	}

	// Verify file was created
	_, err = os.Stat(configPath)
	if os.IsNotExist(err) {
		t.Fatalf("Config file was not created at %s", configPath)
	}

	// Load and verify default values were written
	var loadedConfig Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	err = json.Unmarshal(data, &loadedConfig)
	if err != nil {
		t.Fatalf("Failed to parse config file: %v", err)
	}

	// Check that default values match
	if loadedConfig.Host != defaultConfig.Host {
		t.Errorf("Expected default host '%s', got '%s'", defaultConfig.Host, loadedConfig.Host)
	}
	if loadedConfig.Port != defaultConfig.Port {
		t.Errorf("Expected default port '%s', got '%s'", defaultConfig.Port, loadedConfig.Port)
	}

	// Test case 2: Config file already exists, should not modify it
	customConfig := Config{
		Host: "custom.example.com",
		Port: "9090",
	}

	// Write custom config to file
	customData, err := json.MarshalIndent(customConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal custom config: %v", err)
	}

	err = os.WriteFile(configPath, customData, 0644)
	if err != nil {
		t.Fatalf("Failed to write custom config file: %v", err)
	}

	// Call ensureConfigFileExists again
	err = ensureConfigFileExists(configPath)
	if err != nil {
		t.Fatalf("ensureConfigFileExists failed on existing file: %v", err)
	}

	// Verify file was not modified
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var updatedConfig Config
	err = json.Unmarshal(data, &updatedConfig)
	if err != nil {
		t.Fatalf("Failed to parse updated config file: %v", err)
	}

	// Ensure original custom values are preserved
	if updatedConfig.Host != customConfig.Host {
		t.Errorf("Expected host '%s' to be preserved, got '%s'", customConfig.Host, updatedConfig.Host)
	}
	if updatedConfig.Port != customConfig.Port {
		t.Errorf("Expected port '%s' to be preserved, got '%s'", customConfig.Port, updatedConfig.Port)
	}

	// Test case 3: Create nested directory for config file
	nestedPath := tempDir + "/nested/dir/config.json"

	err = ensureConfigFileExists(nestedPath)
	if err != nil {
		t.Fatalf("ensureConfigFileExists failed with nested path: %v", err)
	}

	// Verify nested directories were created
	_, err = os.Stat(tempDir + "/nested/dir")
	if os.IsNotExist(err) {
		t.Fatalf("Nested directories were not created")
	}

	// Verify file was created
	_, err = os.Stat(nestedPath)
	if os.IsNotExist(err) {
		t.Fatalf("Config file was not created at nested path %s", nestedPath)
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

// TestMainConfigIntegration tests that the main function properly handles config file operations
func TestMainConfigIntegration(t *testing.T) {
	// Create a temporary directory for test
	tempDir, err := os.MkdirTemp("", "config_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a path for a non-existent config file
	testConfigPath := tempDir + "/config.json"

	// Save original variables and restore after test
	origDatabases := databases
	origConfigPath := configPath
	origConfig := config

	// Create empty databases map for this test
	databases = make(map[string]*dbConfig)

	defer func() {
		databases = origDatabases
		configPath = origConfigPath
		config = origConfig
	}()

	// Set config path for test
	configPath = testConfigPath

	// Run the ensureConfigFileExists function and check if it succeeds
	err = ensureConfigFileExists(testConfigPath)
	if err != nil {
		t.Fatalf("ensureConfigFileExists failed: %v", err)
	}

	// Verify the config file was created
	_, err = os.Stat(testConfigPath)
	if os.IsNotExist(err) {
		t.Fatalf("Config file was not created at %s", testConfigPath)
	}

	// Verify loadConfig properly loads the default config
	err = loadConfig(testConfigPath)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// Check that default values were loaded correctly
	if config.Host != defaultConfig.Host {
		t.Errorf("Expected default host '%s', got '%s'", defaultConfig.Host, config.Host)
	}
	if config.Port != defaultConfig.Port {
		t.Errorf("Expected default port '%s', got '%s'", defaultConfig.Port, config.Port)
	}

	// Modify the config and save it
	modifiedConfig := Config{
		Host: "modified.example.com",
		Port: "8888",
	}

	// Write modified config to file
	modifiedData, err := json.MarshalIndent(modifiedConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal modified config: %v", err)
	}

	err = os.WriteFile(testConfigPath, modifiedData, 0644)
	if err != nil {
		t.Fatalf("Failed to write modified config file: %v", err)
	}

	// Run ensureConfigFileExists again - should not modify the existing file
	err = ensureConfigFileExists(testConfigPath)
	if err != nil {
		t.Fatalf("ensureConfigFileExists failed on existing file: %v", err)
	}

	// Load config again
	err = loadConfig(testConfigPath)
	if err != nil {
		t.Fatalf("loadConfig failed after modification: %v", err)
	}

	// Verify modified values were preserved
	if config.Host != modifiedConfig.Host {
		t.Errorf("Expected modified host '%s', got '%s'", modifiedConfig.Host, config.Host)
	}
	if config.Port != modifiedConfig.Port {
		t.Errorf("Expected modified port '%s', got '%s'", modifiedConfig.Port, config.Port)
	}
}

func TestValidateSSLConfig(t *testing.T) {
	// Save original config
	originalConfig := config

	// Restore original config after test
	defer func() {
		config = originalConfig
	}()

	// Test 1: SSL disabled, should pass
	config = Config{
		SSL:  false,
		Cert: "",
		Key:  "",
	}

	err := validateSSLConfig()
	if err != nil {
		t.Errorf("validateSSLConfig failed with SSL disabled: %v", err)
	}

	// Test 2: SSL enabled, but cert and key are empty (will use self-signed)
	// This should pass
	config = Config{
		SSL:  true,
		Cert: "",
		Key:  "",
	}

	err = validateSSLConfig()
	if err != nil {
		t.Errorf("validateSSLConfig failed with SSL enabled and empty cert/key: %v", err)
	}

	// Test 3: SSL enabled, cert specified but key not specified
	// This should fail
	config = Config{
		SSL:  true,
		Cert: "cert.pem",
		Key:  "",
	}

	err = validateSSLConfig()
	if err == nil {
		t.Error("validateSSLConfig should fail with SSL enabled and only cert specified")
	}

	// Test 4: SSL enabled, key specified but cert not specified
	// This should fail
	config = Config{
		SSL:  true,
		Cert: "",
		Key:  "key.pem",
	}

	err = validateSSLConfig()
	if err == nil {
		t.Error("validateSSLConfig should fail with SSL enabled and only key specified")
	}

	// Test 5: SSL enabled, both cert and key specified
	// This should pass
	config = Config{
		SSL:  true,
		Cert: "cert.pem",
		Key:  "key.pem",
	}

	err = validateSSLConfig()
	if err != nil {
		t.Errorf("validateSSLConfig failed with SSL enabled and both cert/key specified: %v", err)
	}
}

// TestGenerateSelfSignedCert tests the certificate generation function
// This is a limited test that doesn't actually check certificate validity
// but ensures files are created
func TestGenerateSelfSignedCert(t *testing.T) {
	// Skip test if openssl is not available
	if _, err := os.Stat("/usr/bin/openssl"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/openssl"); os.IsNotExist(err) {
			t.Skip("Skipping test because openssl is not available")
		}
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "cert_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Change to temp directory
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Return to original directory when done
	defer os.Chdir(currentDir)

	// Generate self-signed certificate
	certFile, keyFile, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("generateSelfSignedCert failed: %v", err)
	}

	// Verify the files were created
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		t.Errorf("Certificate file %s was not created", certFile)
	}

	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		t.Errorf("Key file %s was not created", keyFile)
	}
}

// TestGetClientIPEdgeCases tests additional edge cases for the getClientIP function
func TestGetClientIPEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		request  *http.Request
		expected string
	}{
		{
			name:     "Empty X-Forwarded-For",
			request:  httptest.NewRequest(http.MethodGet, "/", nil),
			expected: "192.0.2.1",
		},
		{
			name:     "Invalid IP in X-Forwarded-For",
			request:  httptest.NewRequest(http.MethodGet, "/", nil),
			expected: "invalid-ip", // Actual implementation doesn't validate the IP
		},
		{
			name:     "X-Real-IP header",
			request:  httptest.NewRequest(http.MethodGet, "/", nil),
			expected: "192.0.2.1", // X-Real-IP is not used in the implementation
		},
		{
			name:     "Invalid remote address",
			request:  httptest.NewRequest(http.MethodGet, "/", nil),
			expected: "invalid-address", // Returns RemoteAddr directly if SplitHostPort fails
		},
	}

	// Setup test cases
	tests[0].request.Header.Set("X-Forwarded-For", "")
	tests[0].request.RemoteAddr = "192.0.2.1:12345"

	tests[1].request.Header.Set("X-Forwarded-For", "invalid-ip")
	tests[1].request.RemoteAddr = "192.0.2.1:12345"

	tests[2].request.Header.Set("X-Real-IP", "10.1.2.3")
	tests[2].request.RemoteAddr = "192.0.2.1:12345"

	tests[3].request.RemoteAddr = "invalid-address"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := getClientIP(tc.request)
			if ip != tc.expected {
				t.Errorf("Expected IP '%s', got '%s'", tc.expected, ip)
			}
		})
	}
}

// ErrorMockReader implements Reader interface but returns errors for all methods
type ErrorMockReader struct{}

func (m *ErrorMockReader) ASN(ip net.IP) (*geoip2.ASN, error) {
	return &geoip2.ASN{
		AutonomousSystemNumber:       12345,
		AutonomousSystemOrganization: "Mock ISP",
	}, nil // Return valid data to continue with the test
}

func (m *ErrorMockReader) City(ip net.IP) (*geoip2.City, error) {
	return nil, fmt.Errorf("mock City error")
}

func (m *ErrorMockReader) Country(ip net.IP) (*geoip2.Country, error) {
	return nil, fmt.Errorf("mock Country error")
}

func (m *ErrorMockReader) Close() error {
	return nil
}

// Test that getIPInfo handles errors from readers correctly
func TestGetIPInfoErrors(t *testing.T) {
	// Save original databases and restore after test
	originalDatabases := databases
	defer func() { databases = originalDatabases }()

	// Setup mock database with error conditions
	errorReader := &ErrorMockReader{}
	databases = map[string]*dbConfig{
		"asn": {
			reader: errorReader,
			mutex:  originalDatabases["asn"].mutex,
		},
		"city": {
			reader: errorReader,
			mutex:  originalDatabases["city"].mutex,
		},
		"country": {
			reader: errorReader,
			mutex:  originalDatabases["country"].mutex,
		},
	}

	// Test with valid IP but readers that return errors
	ip := net.ParseIP("192.168.1.1")
	_, err := getIPInfo(ip)

	// Should return an error due to city lookup failure
	if err == nil {
		t.Fatalf("getIPInfo should have returned an error but got nil")
	}

	// Verify it's the expected error
	if !strings.Contains(err.Error(), "city lookup error") {
		t.Errorf("Expected city lookup error, got: %v", err)
	}
}

// Test handling of invalid IP in handleIPLookup
func TestHandleIPLookupInvalidIP(t *testing.T) {
	// Test with completely invalid string
	req := httptest.NewRequest(http.MethodGet, "/ipgeo/not-an-ip-at-all", nil)
	w := httptest.NewRecorder()

	handleIPLookup(w, req, "not-an-ip-at-all")

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status Bad Request for invalid IP, got %v", resp.Status)
	}

	// Test with malformed IP
	req = httptest.NewRequest(http.MethodGet, "/ipgeo/192.168.1", nil)
	w = httptest.NewRecorder()

	handleIPLookup(w, req, "192.168.1")

	resp = w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status Bad Request for malformed IP, got %v", resp.Status)
	}
}

// TestHandleIPLookupErrors tests error handling in handleIPLookup
func TestHandleIPLookupErrors(t *testing.T) {
	// Save original databases
	originalDatabases := databases

	// Restore original databases after test
	defer func() {
		databases = originalDatabases
	}()

	// Test with error from getIPInfo
	errorReader := &ErrorMockReader{}
	databases = map[string]*dbConfig{
		"asn": {
			reader: errorReader,
			mutex:  originalDatabases["asn"].mutex,
		},
		"city": {
			reader: errorReader,
			mutex:  originalDatabases["city"].mutex,
		},
		"country": {
			reader: errorReader,
			mutex:  originalDatabases["country"].mutex,
		},
	}

	// Set up request for a valid IP
	req := httptest.NewRequest(http.MethodGet, "/ipgeo/192.168.1.1", nil)
	w := httptest.NewRecorder()

	// Call handleIPLookup with a valid IP
	handleIPLookup(w, req, "192.168.1.1")

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status Internal Server Error, got %v", resp.Status)
	}

	// Check response body contains the error message
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "city lookup error") {
		t.Errorf("Expected error message about city lookup, got: %s", string(body))
	}
}

// MockResponseWriter that fails during Write for testing encoding errors
type MockErrorWriter struct {
	http.ResponseWriter
	headers http.Header
}

func NewMockErrorWriter() *MockErrorWriter {
	return &MockErrorWriter{
		headers: make(http.Header),
	}
}

func (m *MockErrorWriter) Header() http.Header {
	return m.headers
}

func (m *MockErrorWriter) Write(data []byte) (int, error) {
	return 0, fmt.Errorf("mock write error")
}

func (m *MockErrorWriter) WriteHeader(statusCode int) {
	// Do nothing
}

// TestHandleIPLookupWriteError tests handling of write/encoding errors in handleIPLookup
func TestHandleIPLookupWriteError(t *testing.T) {
	// Save original databases
	originalDatabases := databases

	// Restore original databases after test
	defer func() {
		databases = originalDatabases
	}()

	// Mock the databases to return valid data
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

	// Set up a normal request but with a response writer that fails
	req := httptest.NewRequest(http.MethodGet, "/ipgeo/192.168.1.1", nil)
	w := NewMockErrorWriter()

	// Call handleIPLookup - this should log an error but not panic
	handleIPLookup(w, req, "192.168.1.1")

	// If we got here without panicking, we're good
}