# Contributing

## Privacy and fixture rules

Use synthetic, sanitized data only. Never commit API keys, cookies, tokens, MAC addresses, IP addresses, controller or site identifiers, personal device names, private topology, raw controller responses, or production output. Invent obvious placeholder IDs and keep fixtures as small as the behavior needs.

If sensitive data is added accidentally, stop and remove it from the full change history before sharing the branch.

## Read-only boundary

This foundation is local-fixture-only. Do not add controller HTTP access, discovery, authentication, write endpoints, mutation methods, or a general UniFi client framework as part of this slice. Future ingestion work must default to read-only operations, use least-privilege credentials, and keep controller responses out of logs and test fixtures.

GitHub issue #2 remains open because live controller HTTP ingestion is not implemented.

## Before submitting

Run the checks documented in README. Review every fixture and command output for private data. Keep changes narrowly scoped and use the standard library unless a dependency is essential.
