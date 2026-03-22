# Network Diagnostics Tool

Runs network diagnostic commands for troubleshooting connectivity, DNS, and routing issues.

## Available Actions
- **ping**: ICMP ping to test host reachability
- **traceroute**: Trace the network path to a destination
- **dns-lookup**: DNS resolution using nslookup, dig, and host
- **port-check**: Test TCP port reachability (format: host:port)
- **connections**: Show listening ports and established connections
- **interfaces**: List network interfaces and their configuration
- **routes**: Display routing table and default gateway
- **bandwidth**: Measure interface bandwidth (1-second snapshot)

## Usage
- Set `action` to choose the diagnostic type
- Set `target` for actions that require a destination (ping, traceroute, dns-lookup, port-check)
- Set `count` to control number of packets/results

## Security
- Read-only diagnostic operations
- Network access is enabled for diagnostic traffic
- Sandboxed execution with 2-minute timeout
- Full audit trail of all diagnostic operations
