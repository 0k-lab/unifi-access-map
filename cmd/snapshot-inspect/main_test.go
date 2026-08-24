package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPrintsSanitizedSummary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"../../internal/snapshot/testdata/degraded-v1.json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}

	want := "schema_version: v1\n" +
		"networks: complete count=1 diagnostics=-\n" +
		"firewall_zones: complete count=0 diagnostics=-\n" +
		"adopted_devices: complete count=0 diagnostics=-\n" +
		"active_clients: partial count=1 diagnostics=incomplete_response\n" +
		"wifi_broadcasts: unavailable count=0 diagnostics=endpoint_unsupported\n" +
		"firewall_policies: complete count=0 diagnostics=-\n" +
		"acl_rules: complete count=0 diagnostics=-\n" +
		"policy_references: complete count=0 diagnostics=-\n"
	if stdout.String() != want {
		t.Fatalf("unexpected output:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "synthetic-") {
		t.Fatal("summary exposed an item ID")
	}
}

func TestRunKeepsInvalidFixtureErrorGeneric(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"/private/path/controller-export.json"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run returned %d", code)
	}
	if stdout.Len() != 0 || stderr.String() != "snapshot-inspect: invalid fixture\n" {
		t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
