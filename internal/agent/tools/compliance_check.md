# Compliance Check Tool

Runs automated compliance checks against industry security frameworks.

## Supported Frameworks
- **cis-linux**: CIS Benchmark for Linux (filesystem, auth, network, logging categories)
- **cis-docker**: CIS Docker Benchmark
- **pci-dss**: PCI Data Security Standard
- **soc2**: SOC 2 Trust Services Criteria
- **hipaa**: HIPAA Security Rule
- **iso27001**: ISO 27001 Information Security

## Usage
- Set `framework` to choose which compliance standard to check against
- Optionally set `category` to check a specific area (e.g., `filesystem`, `auth`, `network`, `logging`)
- Results include PASS/FAIL/WARN counts and an overall compliance score

## Output
Each check reports:
- `[PASS]` - Control requirement met
- `[FAIL]` - Control requirement not met (action required)
- `[WARN]` - Control could not be verified or is partially met

## Security
- Read-only checks only (no system modifications)
- All operations sandboxed with resource limits
- Full audit trail of every compliance check
