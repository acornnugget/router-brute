# router-brute

A fast, concurrent password brute-forcing tool for MikroTik RouterOS devices.

## Features

- **RouterOS v6 & v7 support** - Works with both legacy and modern MikroTik devices
- **Multiple protocols** - Binary API, WebFig, and REST API authentication
- **Concurrent attacks** - High-performance brute-forcing with configurable workers
- **Rate limiting** - Avoid detection with configurable delay between attempts
- **Cross-platform** - Builds for Linux, Windows, and macOS

## Installation

### From Source

```bash
git clone https://github.com/dmonizer/router-brute.git
cd router-brute
make build
```

### Pre-built Binaries

Download from [Releases](https://github.com/dmonizer/router-brute/releases) page.

## Usage

### RouterOS v6 (Binary API)

```bash
./router-brute mikrotik-v6 --target 192.168.1.1 --user admin --wordlist passwords.txt
```

### RouterOS v7 (WebFig)

```bash
./router-brute mikrotik-v7 --target 192.168.1.1 --user admin --wordlist passwords.txt
```

### RouterOS v7 (REST API)

```bash
./router-brute mikrotik-v7-rest --target 192.168.1.1 --user admin --wordlist passwords.txt --https
```

## Options

```
Usage:
  router-brute [command]

Available Commands:
  mikrotik-v6     Attack RouterOS v6 devices using binary API
  mikrotik-v7     Attack RouterOS v7 devices using WebFig protocol
  mikrotik-v7-rest Attack RouterOS v7 devices using REST API

Global Flags:
  --target string     Router IP address or hostname
  --user string       Username to test (default "admin")
  --wordlist string   Path to password wordlist file
  --workers int       Number of concurrent workers (default 5)
  --rate string       Rate limit between attempts (default "100ms")
  --port int          Router API port
  --timeout string    Connection timeout (default "10s")
  --https             Use HTTPS instead of HTTP (REST API only)
```

## Wordlists

The tool expects a plain text file with one password per line:

```
password
123456
admin
secret
```

Example wordlists included:
- `common_passwords.txt` - Common router passwords
- `single_password.txt` - Single password for testing

## Development

```bash
# Install dependencies
go mod tidy

# Run tests
make test

# Build
make build

# Format code
make fmt

# Lint
make lint

# Create release builds
make release
```

## License

MIT

## Disclaimer

This tool is for **authorized security testing only**. Only use on systems you own or have explicit permission to test. Unauthorized use may violate local, state, and federal laws.

**Use responsibly.**