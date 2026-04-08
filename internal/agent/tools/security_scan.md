# Security Scan Tool

Runs vulnerability and security scans against hosts, containers, images, and source code.

## Supported Scanners
- **trivy**: Container image and filesystem vulnerability scanning (CVE database)
- **grype**: Alternative container/filesystem vulnerability scanner
- **lynis**: Host-level security auditing
- **chkrootkit**: Rootkit detection
- **rkhunter**: Rootkit hunter
- **secret-scan**: Source code secret detection (hardcoded passwords, API keys, private keys)

## Usage
- Set `scan_type` to choose the scanner
- Set `target` to specify what to scan:
  - Container image: `nginx:latest`, `myregistry/app:v1.2`
  - File/directory: `/app`, `/etc`
  - Host: `localhost`
- Optionally set `severity` to filter results (CRITICAL, HIGH, MEDIUM, LOW)
- Optionally set `format` for output (table, json, summary)

## Security
- Scans are read-only and do not modify the target
- Executed in sandbox with 2GB memory limit and 10-minute timeout
- Network access is disabled during scans
- Full audit trail for every scan execution
