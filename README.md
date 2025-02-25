# GeoIP API

A simple Go HTTP server that provides geolocation information for IP addresses using MaxMind GeoLite2 databases.

## Features

- Lookup IP geolocation data using MaxMind GeoLite2 databases
- API endpoint similar to ipapi.co format
- JSON output with detailed location information
- Container-ready with Docker support

## Prerequisites

- Go 1.18 or higher
- MaxMind GeoLite2 database files:
  - GeoLite2-ASN.mmdb
  - GeoLite2-City.mmdb
  - GeoLite2-Country.mmdb

## Installation

1. Clone this repository
2. Place your MaxMind GeoLite2 database files in the root directory or specify a custom path with the `GEOIP_DB_DIR` environment variable
3. Install dependencies:

```bash
go mod download
```

4. Build the binary:

```bash
go build -o geoip-api
```

## Usage

### Running the server

```bash
./geoip-api
```

The server will start on port 8080 by default. You can change the port by setting the `PORT` environment variable.

### API Endpoints

- `/{ip}/json/` - Get geolocation information for a specific IP address
- `/json/` - Get geolocation information for the client's IP address

### Sample Request

```
http://localhost:8080/213.25.10.45/json/
```

### Sample Response

```json
{
  "ip": "213.25.10.45",
  "network": "213.25.0.0/19",
  "version": "IPv4",
  "city": "Barłożno",
  "region": "Pomerania",
  "region_code": "22",
  "country": "PL",
  "country_name": "Poland",
  "country_code": "PL",
  "country_code_iso3": "POL",
  "country_capital": "Warsaw",
  "country_tld": ".pl",
  "continent_code": "EU",
  "in_eu": true,
  "postal": "83-225",
  "latitude": 53.7835,
  "longitude": 18.6124,
  "timezone": "Europe/Warsaw",
  "utc_offset": "+0100",
  "country_calling_code": "+48",
  "currency": "PLN",
  "currency_name": "Zloty",
  "languages": "pl",
  "country_area": 312685,
  "country_population": 37978548,
  "asn": "AS5617",
  "org": "Orange Polska Spolka Akcyjna"
}
```

## Docker Support

You can build and run the application using Docker:

1. Build the Docker image:

```bash
docker build -t geoip-api .
```

2. Run the container:

```bash
docker run -p 8080:8080 -v /path/to/your/mmdb/files:/app/db geoip-api
```

## Environment Variables

- `PORT`: The port on which the server listens (default: 8080)
- `GEOIP_DB_DIR`: The directory containing the MaxMind database files (default: current directory)

## Notes

- This application comes with a minimal set of country metadata. For production use, you might want to expand this with a more comprehensive dataset.
- The MaxMind GeoLite2 databases are not included with this repository. You need to download them separately from MaxMind's website.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Disclaimer

This project uses GeoLite2 data created by MaxMind, available from [https://www.maxmind.com](https://www.maxmind.com).
