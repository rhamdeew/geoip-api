package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oschwald/geoip2-golang"
)

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
	CountryCapital    string  `json:"country_capital"`
	CountryTLD        string  `json:"country_tld"`
	ContinentCode     string  `json:"continent_code"`
	InEU              bool    `json:"in_eu"`
	Postal            string  `json:"postal"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	Timezone          string  `json:"timezone"`
	UTCOffset         string  `json:"utc_offset"`
	CountryCallingCode string  `json:"country_calling_code"`
	Currency          string  `json:"currency"`
	CurrencyName      string  `json:"currency_name"`
	Languages         string  `json:"languages"`
	CountryArea       uint64  `json:"country_area"`
	CountryPopulation uint64  `json:"country_population"`
	ASN               string  `json:"asn"`
	Org               string  `json:"org"`
}

// Database paths
var (
	dbASN     *geoip2.Reader
	dbCity    *geoip2.Reader
	dbCountry *geoip2.Reader

	// Country metadata (this would ideally come from another source)
	countryMetadata = map[string]struct {
		Capital    string
		TLD        string
		CallingCode string
		Currency   string
		CurrencyName string
		Languages  string
		Area       uint64
		Population uint64
	}{
		"PL": {
			Capital:     "Warsaw",
			TLD:         ".pl",
			CallingCode: "+48",
			Currency:    "PLN",
			CurrencyName: "Zloty",
			Languages:   "pl",
			Area:        312685,
			Population:  37978548,
		},
		// Add more countries as needed
		"US": {
			Capital:     "Washington D.C.",
			TLD:         ".us",
			CallingCode: "+1",
			Currency:    "USD",
			CurrencyName: "US Dollar",
			Languages:   "en",
			Area:        9372610,
			Population:  331002651,
		},
		"GB": {
			Capital:     "London",
			TLD:         ".uk",
			CallingCode: "+44",
			Currency:    "GBP",
			CurrencyName: "Pound Sterling",
			Languages:   "en",
			Area:        242900,
			Population:  67886011,
		},
		// Add more as needed
	}
)

func main() {
	var err error
	
	// Get database directory from environment or use default
	dbDir := os.Getenv("GEOIP_DB_DIR")
	if dbDir == "" {
		dbDir = "."
	}
	
	// Open ASN database
	dbASN, err = geoip2.Open(filepath.Join(dbDir, "GeoLite2-ASN.mmdb"))
	if err != nil {
		log.Fatalf("Error opening ASN database: %v", err)
	}
	defer dbASN.Close()

	// Open City database
	dbCity, err = geoip2.Open(filepath.Join(dbDir, "GeoLite2-City.mmdb"))
	if err != nil {
		log.Fatalf("Error opening City database: %v", err)
	}
	defer dbCity.Close()

	// Open Country database
	dbCountry, err = geoip2.Open(filepath.Join(dbDir, "GeoLite2-Country.mmdb"))
	if err != nil {
		log.Fatalf("Error opening Country database: %v", err)
	}
	defer dbCountry.Close()

	// Configure server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Set up routes
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/json/", handleJSONRequest)

	// Start the server
	log.Printf("Starting server on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		ipAddress := strings.TrimPrefix(r.URL.Path, "/")
		ipAddress = strings.TrimSuffix(ipAddress, "/json")
		ipAddress = strings.TrimSuffix(ipAddress, "/")
		
		// Handle IP lookup
		handleIPLookup(w, r, ipAddress)
		return
	}
	
	fmt.Fprint(w, "IP Geolocation API. Use /{ip}/json/ to get IP information.")
}

func handleJSONRequest(w http.ResponseWriter, r *http.Request) {
	// Extract IP from the URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	
	ipAddress := ""
	for _, part := range parts {
		if net.ParseIP(part) != nil {
			ipAddress = part
			break
		}
	}
	
	if ipAddress == "" {
		// If no IP in path, use the requester's IP
		ipAddress = getClientIP(r)
	}
	
	handleIPLookup(w, r, ipAddress)
}

func handleIPLookup(w http.ResponseWriter, r *http.Request, ipAddress string) {
	w.Header().Set("Content-Type", "application/json")
	
	// Parse IP address
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		http.Error(w, "Invalid IP address", http.StatusBadRequest)
		return
	}
	
	// Get IP information
	ipInfo, err := getIPInfo(ip)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting IP info: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Return the JSON response
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ipInfo); err != nil {
		log.Printf("Error encoding JSON: %v", err)
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
	asn, err := dbASN.ASN(ip)
	if err != nil {
		return nil, fmt.Errorf("ASN lookup error: %v", err)
	}
	
	info.ASN = fmt.Sprintf("AS%d", asn.AutonomousSystemNumber)
	info.Org = asn.AutonomousSystemOrganization
	
	// Get city information
	city, err := dbCity.City(ip)
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
	country, err := dbCountry.Country(ip)
	if err != nil {
		return nil, fmt.Errorf("country lookup error: %v", err)
	}
	
	info.Country = country.Country.IsoCode
	info.CountryName = country.Country.Names["en"]
	info.CountryCode = country.Country.IsoCode
	info.CountryCodeISO3 = country.Country.IsoCode3
	info.ContinentCode = country.Continent.Code
	info.InEU = country.Country.IsInEuropeanUnion
	
	// Get country metadata if available
	if metadata, ok := countryMetadata[info.CountryCode]; ok {
		info.CountryCapital = metadata.Capital
		info.CountryTLD = metadata.TLD
		info.CountryCallingCode = metadata.CallingCode
		info.Currency = metadata.Currency
		info.CurrencyName = metadata.CurrencyName
		info.Languages = metadata.Languages
		info.CountryArea = metadata.Area
		info.CountryPopulation = metadata.Population
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
	
	// Get network information
	if city.Traits.Network != "" {
		info.Network = city.Traits.Network
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
