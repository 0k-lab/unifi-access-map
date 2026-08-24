# unifi-access-map

A visual map for understanding UniFi network access policies.

This first foundation slice defines a normalized, versioned snapshot and validates synthetic JSON fixtures. It does **not** connect to a UniFi controller, make HTTP requests, mutate controller state, or provide a UI. Live controller HTTP ingestion is not implemented, and GitHub issue #2 remains open.

## Snapshot v1

`internal/snapshot` defines these required sections:

- networks, firewall zones, adopted devices, and active clients
- Wi-Fi broadcasts and firewall policies
- ACL rules and policy references

Every section has a `complete`, `partial`, or `unavailable` status and an explicit non-null `items` array. Complete sections have no diagnostics. Partial and unavailable sections require unique, bounded diagnostic codes, and unavailable sections have an empty `items` array. A missing reference is rejected when its target section is complete; it is tolerated when that section is partial or unavailable because its diagnostics record the uncertainty.

The strict, size-bounded fixture loader rejects duplicate JSON keys at any depth before decoding. Returned errors contain only stable categories and hard-coded section context; they never echo fixture values, identifiers, JSON keys, decoder text, or file paths.

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
