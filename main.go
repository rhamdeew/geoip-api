package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
)

// Define a function type for geoip2.Open to make it mockable in tests
type openFunc func(string) (Reader, error)

// Default implementation uses the real geoip2.Open
var geoipOpen openFunc = func(filename string) (Reader, error) {
	return geoip2.Open(filename)
}

// Config represents the application configuration
type Config struct {
	Host string `json:"host"`
	Port string `json:"port"`
	SSL  bool   `json:"ssl"`   // Whether to use SSL
	Cert string `json:"cert"`  // Path to certificate file
	Key  string `json:"key"`   // Path to key file
}

// Default configuration values
var defaultConfig = Config{
	Host: "",     // Empty host means accept all hosts
	Port: "5324", // Default port
	SSL:  false,  // Default to not using SSL
	Cert: "",     // Empty means no certificate file
	Key:  "",     // Empty means no key file
}

// IPInfo represents the information about an IP address
type IPInfo struct {
	IP                string  `json:"ip"`
	Network           string  `json:"network"`
	Version           string  `json:"version"`
	City              string  `json:"city"`
	Region            string  `json:"region"`
	RegionCode        string  `json:"region_code"`
	Country           string  `json:"country"`
	CountryName       string  `json:"country_name"`
	CountryCode       string  `json:"country_code"`
	CountryCodeISO3   string  `json:"country_code_iso3"`
	ContinentCode     string  `json:"continent_code"`
	InEU              bool    `json:"in_eu"`
	Postal            string  `json:"postal"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	Timezone          string  `json:"timezone"`
	UTCOffset         string  `json:"utc_offset"`
	ASN               string  `json:"asn"`
	Org               string  `json:"org"`
}

// Reader interface provides a common interface for GeoIP functionality
type Reader interface {
	ASN(net.IP) (*geoip2.ASN, error)
	City(net.IP) (*geoip2.City, error)
	Country(net.IP) (*geoip2.Country, error)
	Close() error
}

// Database configuration
type dbConfig struct {
	reader     Reader
	url        string
	localPath  string
	lastUpdate time.Time
	mutex      sync.RWMutex
}

// Application configuration
var (
	config     Config
	configPath string
)

// Database readers and configuration
var (
	// Fixed path for MaxMind databases
	dbDir = "./maxmind_db"

	// Database configurations
	databases = map[string]*dbConfig{
		"asn": {
			url:       "https://git.io/GeoLite2-ASN.mmdb",
			localPath: filepath.Join(dbDir, "GeoLite2-ASN.mmdb"),
		},
		"city": {
			url:       "https://git.io/GeoLite2-City.mmdb",
			localPath: filepath.Join(dbDir, "GeoLite2-City.mmdb"),
		},
		"country": {
			url:       "https://git.io/GeoLite2-Country.mmdb",
			localPath: filepath.Join(dbDir, "GeoLite2-Country.mmdb"),
		},
	}

	// ISO3 country codes mapping
	iso3Codes = map[string]string{
		"AD": "AND", "AE": "ARE", "AF": "AFG", "AG": "ATG", "AI": "AIA", "AL": "ALB", "AM": "ARM",
		"AO": "AGO", "AQ": "ATA", "AR": "ARG", "AS": "ASM", "AT": "AUT", "AU": "AUS", "AW": "ABW",
		"AX": "ALA", "AZ": "AZE", "BA": "BIH", "BB": "BRB", "BD": "BGD", "BE": "BEL", "BF": "BFA",
		"BG": "BGR", "BH": "BHR", "BI": "BDI", "BJ": "BEN", "BL": "BLM", "BM": "BMU", "BN": "BRN",
		"BO": "BOL", "BQ": "BES", "BR": "BRA", "BS": "BHS", "BT": "BTN", "BV": "BVT", "BW": "BWA",
		"BY": "BLR", "BZ": "BLZ", "CA": "CAN", "CC": "CCK", "CD": "COD", "CF": "CAF", "CG": "COG",
		"CH": "CHE", "CI": "CIV", "CK": "COK", "CL": "CHL", "CM": "CMR", "CN": "CHN", "CO": "COL",
		"CR": "CRI", "CU": "CUB", "CV": "CPV", "CW": "CUW", "CX": "CXR", "CY": "CYP", "CZ": "CZE",
		"DE": "DEU", "DJ": "DJI", "DK": "DNK", "DM": "DMA", "DO": "DOM", "DZ": "DZA", "EC": "ECU",
		"EE": "EST", "EG": "EGY", "EH": "ESH", "ER": "ERI", "ES": "ESP", "ET": "ETH", "FI": "FIN",
		"FJ": "FJI", "FK": "FLK", "FM": "FSM", "FO": "FRO", "FR": "FRA", "GA": "GAB", "GB": "GBR",
		"GD": "GRD", "GE": "GEO", "GF": "GUF", "GG": "GGY", "GH": "GHA", "GI": "GIB", "GL": "GRL",
		"GM": "GMB", "GN": "GIN", "GP": "GLP", "GQ": "GNQ", "GR": "GRC", "GS": "SGS", "GT": "GTM",
		"GU": "GUM", "GW": "GNB", "GY": "GUY", "HK": "HKG", "HM": "HMD", "HN": "HND", "HR": "HRV",
		"HT": "HTI", "HU": "HUN", "ID": "IDN", "IE": "IRL", "IL": "ISR", "IM": "IMN", "IN": "IND",
		"IO": "IOT", "IQ": "IRQ", "IR": "IRN", "IS": "ISL", "IT": "ITA", "JE": "JEY", "JM": "JAM",
		"JO": "JOR", "JP": "JPN", "KE": "KEN", "KG": "KGZ", "KH": "KHM", "KI": "KIR", "KM": "COM",
		"KN": "KNA", "KP": "PRK", "KR": "KOR", "KW": "KWT", "KY": "CYM", "KZ": "KAZ", "LA": "LAO",
		"LB": "LBN", "LC": "LCA", "LI": "LIE", "LK": "LKA", "LR": "LBR", "LS": "LSO", "LT": "LTU",
		"LU": "LUX", "LV": "LVA", "LY": "LBY", "MA": "MAR", "MC": "MCO", "MD": "MDA", "ME": "MNE",
		"MF": "MAF", "MG": "MDG", "MH": "MHL", "MK": "MKD", "ML": "MLI", "MM": "MMR", "MN": "MNG",
		"MO": "MAC", "MP": "MNP", "MQ": "MTQ", "MR": "MRT", "MS": "MSR", "MT": "MLT", "MU": "MUS",
		"MV": "MDV", "MW": "MWI", "MX": "MEX", "MY": "MYS", "MZ": "MOZ", "NA": "NAM", "NC": "NCL",
		"NE": "NER", "NF": "NFK", "NG": "NGA", "NI": "NIC", "NL": "NLD", "NO": "NOR", "NP": "NPL",
		"NR": "NRU", "NU": "NIU", "NZ": "NZL", "OM": "OMN", "PA": "PAN", "PE": "PER", "PF": "PYF",
		"PG": "PNG", "PH": "PHL", "PK": "PAK", "PL": "POL", "PM": "SPM", "PN": "PCN", "PR": "PRI",
		"PS": "PSE", "PT": "PRT", "PW": "PLW", "PY": "PRY", "QA": "QAT", "RE": "REU", "RO": "ROU",
		"RS": "SRB", "RU": "RUS", "RW": "RWA", "SA": "SAU", "SB": "SLB", "SC": "SYC", "SD": "SDN",
		"SE": "SWE", "SG": "SGP", "SH": "SHN", "SI": "SVN", "SJ": "SJM", "SK": "SVK", "SL": "SLE",
		"SM": "SMR", "SN": "SEN", "SO": "SOM", "SR": "SUR", "SS": "SSD", "ST": "STP", "SV": "SLV",
		"SX": "SXM", "SY": "SYR", "SZ": "SWZ", "TC": "TCA", "TD": "TCD", "TF": "ATF", "TG": "TGO",
		"TH": "THA", "TJ": "TJK", "TK": "TKL", "TL": "TLS", "TM": "TKM", "TN": "TUN", "TO": "TON",
		"TR": "TUR", "TT": "TTO", "TV": "TUV", "TW": "TWN", "TZ": "TZA", "UA": "UKR", "UG": "UGA",
		"UM": "UMI", "US": "USA", "UY": "URY", "UZ": "UZB", "VA": "VAT", "VC": "VCT", "VE": "VEN",
		"VG": "VGB", "VI": "VIR", "VN": "VNM", "VU": "VUT", "WF": "WLF", "WS": "WSM", "YE": "YEM",
		"YT": "MYT", "ZA": "ZAF", "ZM": "ZMB", "ZW": "ZWE",
	}
)

func init() {
	// Define command line flags
	flag.StringVar(&configPath, "config", "config.json", "Path to configuration file")
}

func main() {
	// Parse command line flags
	flag.Parse()

	// Check if config.json exists, create it if it doesn't
	if err := ensureConfigFileExists(configPath); err != nil {
		log.Printf("Error creating configuration file: %v", err)
	}

	// Load configuration
	if err := loadConfig(configPath); err != nil {
		log.Printf("Error loading configuration: %v", err)
		log.Println("Using default configuration")

		// Set default configuration
		config = defaultConfig
	}

	// Validate SSL configuration
	if err := validateSSLConfig(); err != nil {
		log.Fatalf("Invalid SSL configuration: %v", err)
	}

	// Ensure database directory exists
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Initialize or update databases
	if err := initDatabases(); err != nil {
		log.Fatalf("Error initializing databases: %v", err)
	}

	// Start a goroutine to periodically update databases
	go startDatabaseUpdater()

	// Set up router with custom handler that checks all requests
	http.HandleFunc("/", handleRequest)

	// Start the server
	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)
	log.Printf("Starting server on %s...\n", addr)

	if config.SSL {
		// Ensure we have certificate and key files
		if config.Cert == "" || config.Key == "" {
			// Generate self-signed certificates
			certFile, keyFile, err := generateSelfSignedCert()
			if err != nil {
				log.Fatalf("Failed to generate self-signed certificate: %v", err)
			}
			config.Cert = certFile
			config.Key = keyFile
			log.Printf("Using self-signed certificate: %s and key: %s", config.Cert, config.Key)
		} else {
			log.Printf("Using provided certificate: %s and key: %s", config.Cert, config.Key)
		}

		log.Fatal(http.ListenAndServeTLS(addr, config.Cert, config.Key, nil))
	} else {
		log.Fatal(http.ListenAndServe(addr, nil))
	}
}

// Ensure the configuration file exists, create with default values if it doesn't
func ensureConfigFileExists(path string) error {
	// Ensure parent directory exists
	configDir := filepath.Dir(path)
	if configDir != "." && configDir != "/" {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory %s: %v", configDir, err)
		}
	}

	// Check if file exists
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		log.Printf("Configuration file %s does not exist, creating with default values", path)

		// Create the file with default config
		data, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal default configuration: %v", err)
		}

		err = os.WriteFile(path, data, 0644)
		if err != nil {
			return fmt.Errorf("failed to write default configuration file: %v", err)
		}

		log.Printf("Created default configuration file at %s", path)
	} else if err != nil {
		return fmt.Errorf("failed to check if config file exists: %v", err)
	}

	return nil
}

// Load configuration from file
func loadConfig(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	log.Printf("Configuration loaded from %s", path)
	log.Printf("  Host: %s", config.Host)
	log.Printf("  Port: %s", config.Port)
	log.Printf("  SSL: %v", config.SSL)
	if config.SSL {
		log.Printf("  Certificate: %s", config.Cert)
		log.Printf("  Key: %s", config.Key)
	}

	return nil
}

// Initialize databases - download if needed and open readers
func initDatabases() error {
	for name, db := range databases {
		// Check if database file exists
		if _, err := os.Stat(db.localPath); os.IsNotExist(err) {
			// Database file doesn't exist, download it
			log.Printf("Database %s not found, downloading...", name)
			if err := downloadDatabase(db.url, db.localPath); err != nil {
				return fmt.Errorf("failed to download %s database: %v", name, err)
			}
			db.lastUpdate = time.Now()
		}

		// Open the database reader
		reader, err := geoipOpen(db.localPath)
		if err != nil {
			return fmt.Errorf("error opening %s database: %v", name, err)
		}

		log.Printf("Successfully opened %s database", name)
		db.reader = reader

		// If we don't know when it was last updated, set to file's modification time
		if db.lastUpdate.IsZero() {
			if info, err := os.Stat(db.localPath); err == nil {
				db.lastUpdate = info.ModTime()
			} else {
				// If we can't get mod time, just use now
				db.lastUpdate = time.Now()
			}
		}
	}

	return nil
}

// Start a goroutine that periodically updates the databases
func startDatabaseUpdater() {
	ticker := time.NewTicker(24 * time.Hour) // Check daily
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			updateDatabasesIfNeeded()
		}
	}
}

// Check if databases need updating and update them if needed
func updateDatabasesIfNeeded() {
	for name, db := range databases {
		// Check if database is older than one month
		if time.Since(db.lastUpdate) >= 30*24*time.Hour {
			log.Printf("Database %s is older than 30 days, updating...", name)

			// Download to a temporary file
			tempPath := db.localPath + ".new"
			if err := downloadDatabase(db.url, tempPath); err != nil {
				log.Printf("Failed to download updated %s database: %v", name, err)
				continue
			}

			// Close the existing reader before replacing the file
			db.mutex.Lock()
			if db.reader != nil {
				db.reader.Close()
			}

			// Replace the old file with the new one
			if err := os.Rename(tempPath, db.localPath); err != nil {
				log.Printf("Failed to replace %s database file: %v", name, err)
				// Try to reopen the old file
				if reader, err := geoip2.Open(db.localPath); err == nil {
					db.reader = reader
				}
				db.mutex.Unlock()
				continue
			}

			// Open the new database
			reader, err := geoipOpen(db.localPath)
			if err != nil {
				log.Printf("Failed to open updated %s database: %v", name, err)
				db.mutex.Unlock()
				continue
			}

			// Update the reader and last update time
			db.reader = reader
			db.lastUpdate = time.Now()
			db.mutex.Unlock()

			log.Printf("Successfully updated %s database", name)
		}
	}
}

// Download a file from the specified URL to the local path
func downloadDatabase(url string, localPath string) error {
	// Create a temporary file
	out, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Send HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Copy the file content
	_, err = io.Copy(out, resp.Body)
	return err
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Check host if configured
	if config.Host != "" {
		// Parse the Host header to extract hostname without port
		requestHost := r.Host
		if hostWithoutPort, _, err := net.SplitHostPort(requestHost); err == nil {
			// If we could split the host and port, use just the host part
			requestHost = hostWithoutPort
		}

		if requestHost != config.Host {
			log.Printf("Request rejected due to incorrect host: %s (expected %s)", requestHost, config.Host)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	// Log the request
	log.Printf("Request received: %s %s from %s", r.Method, path, getClientIP(r))

	// Check if path is one of our valid endpoints
	if path == "/ipgeo" {
		// Handle client IP lookup
		clientIP := getClientIP(r)
		log.Printf("Processing request for client IP: %s", clientIP)
		handleIPLookup(w, r, clientIP)
		return
	} else if strings.HasPrefix(path, "/ipgeo/") {
		// Extract IP from the path
		parts := strings.Split(path, "/")
		if len(parts) == 3 && parts[1] == "ipgeo" {
			ipAddress := parts[2]
			log.Printf("Processing request for specific IP: %s", ipAddress)
			handleIPLookup(w, r, ipAddress)
			return
		}
	}

	// All other requests are forbidden
	log.Printf("Rejecting request with 403 Forbidden: %s", path)
	http.Error(w, "Forbidden", http.StatusForbidden)
}

func handleIPLookup(w http.ResponseWriter, r *http.Request, ipAddress string) {
	w.Header().Set("Content-Type", "application/json")

	// Parse IP address
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		log.Printf("Invalid IP address provided: %s", ipAddress)
		http.Error(w, "Invalid IP address", http.StatusBadRequest)
		return
	}

	// Get IP information
	ipInfo, err := getIPInfo(ip)
	if err != nil {
		log.Printf("Error getting info for IP %s: %v", ipAddress, err)
		http.Error(w, fmt.Sprintf("Error getting IP info: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully processed IP %s (%s, %s)",
		ipAddress, ipInfo.CountryName, ipInfo.City)

	// Return the JSON response
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ipInfo); err != nil {
		log.Printf("Error encoding JSON response for IP %s: %v", ipAddress, err)
	}
}

func getIPInfo(ip net.IP) (*IPInfo, error) {
	info := &IPInfo{
		IP:      ip.String(),
		Version: "IPv4",
	}

	if ip.To4() == nil {
		info.Version = "IPv6"
	}

	// Get ASN information
	databases["asn"].mutex.RLock()
	asn, err := databases["asn"].reader.ASN(ip)
	databases["asn"].mutex.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("ASN lookup error: %v", err)
	}

	info.ASN = fmt.Sprintf("AS%d", asn.AutonomousSystemNumber)
	info.Org = asn.AutonomousSystemOrganization

	// Get city information
	databases["city"].mutex.RLock()
	city, err := databases["city"].reader.City(ip)
	databases["city"].mutex.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("city lookup error: %v", err)
	}

	info.City = city.City.Names["en"]
	if len(city.Subdivisions) > 0 {
		info.Region = city.Subdivisions[0].Names["en"]
		info.RegionCode = city.Subdivisions[0].IsoCode
	}
	info.Postal = city.Postal.Code
	info.Latitude = city.Location.Latitude
	info.Longitude = city.Location.Longitude
	info.Timezone = city.Location.TimeZone

	// Get country information
	databases["country"].mutex.RLock()
	country, err := databases["country"].reader.Country(ip)
	databases["country"].mutex.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("country lookup error: %v", err)
	}

	info.Country = country.Country.IsoCode
	info.CountryName = country.Country.Names["en"]
	info.CountryCode = country.Country.IsoCode
	info.ContinentCode = country.Continent.Code
	info.InEU = country.Country.IsInEuropeanUnion

	// MaxMind doesn't provide ISO3 codes directly, so we'll have to populate this from our own data
	if iso3, ok := iso3Codes[info.CountryCode]; ok {
		info.CountryCodeISO3 = iso3
	} else {
		info.CountryCodeISO3 = info.CountryCode // Fallback
	}

	// Calculate UTC offset based on timezone
	if info.Timezone != "" {
		loc, err := time.LoadLocation(info.Timezone)
		if err == nil {
			_, offset := time.Now().In(loc).Zone()
			offsetHours := offset / 3600
			offsetSign := "+"
			if offsetHours < 0 {
				offsetSign = "-"
				offsetHours = -offsetHours
			}
			info.UTCOffset = fmt.Sprintf("%s%02d00", offsetSign, offsetHours)
		}
	}

	// Network information isn't directly available in current version
	// We'll construct a basic network from the IP
	if ip.To4() != nil {
		// For IPv4, use a /24 network as a basic approximation
		network := ip.Mask(net.CIDRMask(24, 32))
		info.Network = fmt.Sprintf("%s/24", network.String())
	} else {
		// For IPv6, use a /64 network as a basic approximation
		network := ip.Mask(net.CIDRMask(64, 128))
		info.Network = fmt.Sprintf("%s/64", network.String())
	}

	return info, nil
}

func getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// The first IP in the list is the client IP
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Otherwise, use RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// validateSSLConfig validates the SSL configuration
func validateSSLConfig() error {
	// If SSL is disabled but cert or key is specified, return an error
	if !config.SSL && (config.Cert != "" || config.Key != "") {
		return fmt.Errorf("SSL is disabled but certificate or key path is provided")
	}

	// If SSL is enabled and only one of cert or key is specified, return an error
	if config.SSL && ((config.Cert != "" && config.Key == "") || (config.Cert == "" && config.Key != "")) {
		return fmt.Errorf("both certificate and key must be provided when using SSL with custom certificates")
	}

	return nil
}

// generateSelfSignedCert generates a self-signed certificate and key
func generateSelfSignedCert() (string, string, error) {
	// Create directory for certificates if it doesn't exist
	certDir := "./certs"
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create certificates directory: %v", err)
	}

	certFile := filepath.Join(certDir, "server.crt")
	keyFile := filepath.Join(certDir, "server.key")

	// Check if files already exist
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			// Both files exist, reuse them
			return certFile, keyFile, nil
		}
	}

	// Generate a new certificate and key using openssl
	log.Println("Generating self-signed certificate...")

	// Generate private key
	keyCmd := exec.Command("openssl", "genrsa", "-out", keyFile, "2048")
	if err := keyCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %v", err)
	}

	// Generate self-signed certificate
	certCmd := exec.Command("openssl", "req", "-new", "-x509", "-key", keyFile,
		"-out", certFile, "-days", "365", "-subj",
		"/C=US/ST=State/L=City/O=Organization/OU=Unit/CN=localhost")
	if err := certCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate self-signed certificate: %v", err)
	}

	log.Println("Self-signed certificate generated successfully")
	return certFile, keyFile, nil
}
