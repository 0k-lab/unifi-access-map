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

## Access graph projection

`accessgraph.Project(s)` in `internal/accessgraph` validates a `snapshot.Snapshot` and returns a graph plus an error. The graph contains typed nodes for networks, firewall zones, adopted devices, active clients, Wi-Fi broadcasts, firewall policies, and ACL rules. Node identity is the pair `(kind, ID)`.

Directed, typed edges represent only explicit snapshot references: zone/device/client/Wi-Fi/ACL to network, client to device, policy to source or destination zone, and policy to ACL through policy references. Both endpoints must exist; missing endpoints tolerated by incomplete sections produce no edge or placeholder node. Repeated relationships collapse to one edge. These references do not calculate effective access or discover live topology.

Nodes sort by kind then ID; edges sort by kind, source kind/ID, then target kind/ID. `IncompleteSections` lists all partial or unavailable source sections (including policy references) in lexical order. Empty lists serialize as `[]`. Projection leaves the snapshot unchanged, and equivalent item/reference orderings produce byte-for-byte identical JSON when marshaled with `encoding/json`.

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
