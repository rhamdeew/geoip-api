package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
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

// Database readers
var (
	dbASN     *geoip2.Reader
	dbCity    *geoip2.Reader
	dbCountry *geoip2.Reader
	
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

func main() {
	var err error
	
	// Fixed path for MaxMind databases
	dbDir := "./maxmind_db"
	
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
	port := "5324"

	// Set up router with custom handler that checks all requests
	http.HandleFunc("/", handleRequest)

	// Start the server
	log.Printf("Starting server on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	
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
