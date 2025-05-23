# GeoIP API Service

![CI/CD Status](https://github.com/rhamdeew/geoip-api/actions/workflows/release.yml/badge.svg)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/rhamdeew/geoip-api)](https://github.com/rhamdeew/geoip-api/releases/latest)
[![GitHub license](https://img.shields.io/github/license/rhamdeew/geoip-api)](https://github.com/rhamdeew/geoip-api/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/rhamdeew/geoip-api)](https://goreportcard.com/report/github.com/rhamdeew/geoip-api)
[![GitHub stars](https://img.shields.io/github/stars/rhamdeew/geoip-api)](https://github.com/rhamdeew/geoip-api/stargazers)

A simple API service that provides geolocation information for IP addresses using MaxMind GeoIP databases.

## Features

- IP lookup with detailed geolocation information
- Support for both IPv4 and IPv6 addresses
- Automatic database updates
- Simple configuration
- Auto-generation of config.json if not exists
- Automatic downloading of MaxMind DB files if not exists

## Usage

### Configuration

A `config.json` file with the following format is used:

```json
{
  "host": "localhost",
  "port": "5324"
}
```

- `host`: The host to bind to (empty string for all interfaces)
- `port`: The port to listen on

If the configuration file doesn't exist, it will be automatically created with default values when the service starts.

### Starting the Service

Run the service:

```
./geoip-api
```

Or specify a custom configuration file:

```
./geoip-api -config /path/to/config.json
```

The service will automatically download the necessary MaxMind GeoIP databases if they don't exist when it starts.

### API Endpoints

- `GET /ipgeo`: Returns information about the client's IP address
- `GET /ipgeo/{ip}`: Returns information about the specified IP address

Example response:

```json
{
  "ip": "8.8.8.8",
  "network": "8.8.8.0/24",
  "version": "IPv4",
  "city": "Mountain View",
  "region": "California",
  "region_code": "CA",
  "country": "US",
  "country_name": "United States",
  "country_code": "US",
  "country_code_iso3": "USA",
  "continent_code": "NA",
  "in_eu": false,
  "postal": "94035",
  "latitude": 37.4056,
  "longitude": -122.0775,
  "timezone": "America/Los_Angeles",
  "utc_offset": "-0700",
  "asn": "AS15169",
  "org": "Google LLC"
}
```

## Installation

### Using the Install Script

The service can be installed as a systemd service using the provided installation script:

```bash
sudo ./install.sh
```

This will:
- Install the service to `/opt/geoip-api`
- Create a systemd service file
- Create a system user and group (`geoip`)
- Configure the service to start on boot

#### Installation Options

The install script supports several options:

```bash
sudo ./install.sh [OPTIONS]
```

Options:
- `--install-dir=DIR`: Installation directory (default: /opt/geoip-api)
- `--host=HOST`: Host to bind to (default: empty, binds to all interfaces)
- `--port=PORT`: Port to listen on (default: 5324)
- `--user=USER`: User to run the service as (default: geoip)
- `--group=GROUP`: Group to run the service as (default: geoip)

### Starting the Service

After installation, you can manage the service using systemctl:

```bash
sudo systemctl enable geoip-api   # Enable service on boot
sudo systemctl start geoip-api    # Start the service
sudo systemctl status geoip-api   # Check service status
```

### Uninstallation

To uninstall the service:

```bash
sudo ./uninstall.sh
```

This will:
- Stop and disable the systemd service
- Remove the service files
- Remove the installation directory

#### Uninstallation Options

```bash
sudo ./uninstall.sh [OPTIONS]
```

Options:
- `--install-dir=DIR`: Installation directory (default: /opt/geoip-api)
- `--user=USER`: User the service runs as (default: geoip)
- `--remove-user`: Also remove the service user account

## Development

### Building

To build the application:

```
make build
```

Or build and run in one step:

```
make run
```

### Testing

Run all tests:

```
make test
```

The tests include:
- Unit tests for all core functions
- HTTP handler tests
- Database management tests

Mock implementations are used for the GeoIP database readers to avoid dependencies on actual MaxMind databases during testing.

### Test Coverage

Check test coverage:

```
make test-coverage
```

This will generate a coverage report in HTML format.
