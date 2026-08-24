# unifi-access-map

A visual map for understanding UniFi network access policies.

This first foundation slice defines a normalized, versioned snapshot and validates synthetic JSON fixtures. It does **not** connect to a UniFi controller, make HTTP requests, mutate controller state, or provide a UI. Live controller HTTP ingestion is not implemented, and GitHub issue #2 remains open.

## Snapshot v1

`internal/snapshot` defines these required sections:

- networks, firewall zones, adopted devices, and active clients
- Wi-Fi broadcasts and firewall policies
- ACL rules and policy references

Every section has a `complete`, `partial`, or `unavailable` status. Partial and unavailable sections require one or more bounded diagnostic codes. The strict fixture loader rejects unknown fields, unsupported versions, invalid statuses or diagnostics, duplicate IDs, and broken normalized references.

The repository contains synthetic fixtures only. Their identifiers are invented placeholders and contain no controller/site identifiers, addresses, credentials, personal device names, private topology, or production output.

## Inspect a fixture

```sh
go run ./cmd/snapshot-inspect internal/snapshot/testdata/complete-v1.json
```

The command reads one local fixture and prints only the schema version plus each section's status, item count, and diagnostic codes. It contains no network or mutation code and never prints item IDs.

## Development checks

```sh
test -z "$(gofmt -l .)"
go test ./...
go vet ./...
go build ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) before adding fixtures or ingestion code.
