# Certificate Audit Tool

Inspects, validates, and audits TLS/SSL certificates for expiry, chain validity, and configuration.

## Available Actions
- **check-expiry**: Check certificate expiration date and warn if approaching
- **inspect**: Display detailed certificate information (subject, issuer, SANs, key usage)
- **verify-chain**: Verify the certificate trust chain
- **scan-dir**: Scan a directory for all certificates and report their status
- **check-host**: Check a remote host's TLS certificate (connects to host:port)

## Usage
- Set `action` to choose the audit type
- Set `target` to:
  - Certificate file path (for check-expiry, inspect, verify-chain)
  - Directory path (for scan-dir)
  - Hostname or hostname:port (for check-host, defaults to port 443)
- Set `warn_days` for expiry warning threshold (default: 30 days)

## Security
- Read-only certificate inspection (no modifications)
- Network access only enabled for check-host action
- Sandboxed execution with resource limits
- Full audit trail of all certificate operations
