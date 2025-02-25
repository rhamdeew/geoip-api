# GeoIP API Installation and Deployment Guide

This guide explains how to install and deploy the GeoIP API service using systemd.

## Building the Application

1. Ensure you have Go 1.18 or later installed:

```bash
go version
```

2. Clone the repository:

```bash
git clone https://github.com/rhamdeew/geoip-api.git
cd geoip-api
```

3. Build the application:

```bash
go mod tidy
go build -o geoip-api
```

## Creating a Service User

Create a dedicated user for running the service:

```bash
sudo useradd -r -s /bin/false geoip
```

## Installation

1. Create the installation directory:

```bash
sudo mkdir -p /opt/geoip-api
sudo mkdir -p /opt/geoip-api/maxmind_db
```

2. Copy the binary, config file, and set permissions:

```bash
sudo cp geoip-api /opt/geoip-api/
sudo cp config.json /opt/geoip-api/
sudo chown -R geoip:geoip /opt/geoip-api
sudo chmod 755 /opt/geoip-api/geoip-api
```

3. Copy the systemd service file:

```bash
sudo cp geoip-api.service /etc/systemd/system/
```

## Starting the Service

1. Reload systemd to recognize the new service:

```bash
sudo systemctl daemon-reload
```

2. Enable the service to start at boot:

```bash
sudo systemctl enable geoip-api
```

3. Start the service:

```bash
sudo systemctl start geoip-api
```

4. Check the service status:

```bash
sudo systemctl status geoip-api
```

## Configuration

The service uses a JSON configuration file located at `/opt/geoip-api/config.json` with the following structure:

```json
{
  "host": "api.example.com",  // Hostname to accept requests from (empty for all hosts)
  "port": "5324"              // Port to listen on
}
```

### Configuration Options

- **host**: Hostname to accept connections from. If set, the API will only respond to requests with a matching `Host` header. Leave empty to accept all hosts.
- **port**: Port number the API should listen on.

You can also specify an alternative configuration file path using the `-config` flag:

```bash
sudo systemctl edit geoip-api
```

Add the following to change the config path:

```
[Service]
ExecStart=
ExecStart=/opt/geoip-api/geoip-api -config /path/to/your/config.json
```

## Usage

The GeoIP API service will:

1. Automatically download MaxMind databases if they're not present
2. Update databases monthly
3. Listen on the configured port and accept requests from the configured hostname

Available endpoints:

- `/ipgeo` - Get geolocation information for the client's IP address
- `/ipgeo/{ip}` - Get geolocation information for a specific IP address

Example:

```
curl http://localhost:5324/ipgeo/8.8.8.8
```

## Logs

View service logs:

```bash
sudo journalctl -u geoip-api
```

## Firewall Configuration

If you have a firewall enabled, allow traffic to port 5324:

### For UFW:

```bash
sudo ufw allow 5324/tcp
```

### For firewalld:

```bash
sudo firewall-cmd --permanent --add-port=5324/tcp
sudo firewall-cmd --reload
```

## Troubleshooting

- If the service fails to start, check the logs:

```bash
sudo journalctl -u geoip-api -n 50 --no-pager
```

- Verify database files exist in the correct location:

```bash
ls -la /opt/geoip-api/maxmind_db/
```

- Check if the service is running and listening on the correct port:

```bash
sudo ss -tulpn | grep 5324
```
