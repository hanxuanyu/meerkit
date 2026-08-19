# Meerkit Network

The official `meerkit.network` plugin provides five independent monitor modules in one managed plugin process:

| Module type | Purpose |
| --- | --- |
| `http` | HTTP/HTTPS requests, response metadata, bodies, and hashes |
| `tcp` | TCP connectivity with optional payload write and response read |
| `dns` | A, AAAA, CNAME, MX, TXT, NS, SRV, CAA, and PTR queries against an explicit DNS server |
| `tls-certificate` | Direct TLS handshakes, negotiated parameters, certificate validation, fingerprints, and expiry |
| `icmp` | IPv4/IPv6 ICMP reachability, packet loss, RTT, and jitter |

DNS auto transport starts with UDP and retries truncated responses over TCP. TLS inspection retrieves the peer certificate before independently evaluating hostname, chain, and time validity, so invalid certificates still produce useful structured results. ICMP auto mode prefers unprivileged Ping Sockets and falls back to Raw Sockets; Raw Socket use may require `CAP_NET_RAW` in containers.

Build and test:

```bash
cd plugins/network
go test ./...
go build ./...
```

Thresholds and expected values belong in Meerkit conditions, such as a DNS RCODE other than `NOERROR`, fewer than 30 certificate days remaining, or ICMP packet loss above 20 percent.
