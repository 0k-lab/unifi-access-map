package snapshot

import (
	"os"
	"strings"
	"testing"
)

func TestLoadCompleteFixture(t *testing.T) {
	value, err := LoadFile("testdata/complete-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if value.SchemaVersion != SchemaVersion || value.Networks.Status != StatusComplete || len(value.PolicyReferences.Items) != 1 {
		t.Fatalf("unexpected complete snapshot: %+v", value)
	}
}

func TestLoadExplicitPartialAndUnavailableSections(t *testing.T) {
	value, err := LoadFile("testdata/degraded-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if value.ActiveClients.Status != StatusPartial || value.ActiveClients.Diagnostics[0] != DiagnosticIncompleteResponse {
		t.Fatalf("unexpected partial section: %+v", value.ActiveClients)
	}
	if value.WiFiBroadcasts.Status != StatusUnavailable || value.WiFiBroadcasts.Diagnostics[0] != DiagnosticEndpointUnsupported {
		t.Fatalf("unexpected unavailable section: %+v", value.WiFiBroadcasts)
	}
}

func TestLoadRejectsInvalidFixtures(t *testing.T) {
	fixture, err := os.ReadFile("testdata/complete-v1.json")
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"unsupported version": strings.Replace(string(fixture), `"v1"`, `"v2"`, 1),
		"unknown field":       strings.Replace(string(fixture), `"schema_version": "v1"`, `"schema_version": "v1", "extra": true`, 1),
		"invalid status":      strings.Replace(string(fixture), `"status": "complete"`, `"status": "stale"`, 1),
		"missing diagnostic":  strings.Replace(string(fixture), `"status": "complete"`, `"status": "partial"`, 1),
		"duplicate ID":        strings.Replace(string(fixture), `[{"id": "synthetic-network-01"}]`, `[{"id": "synthetic-network-01"}, {"id": "synthetic-network-01"}]`, 1),
		"broken reference":    strings.Replace(string(fixture), `"network_id": "synthetic-network-01"`, `"network_id": "synthetic-network-missing"`, 1),
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(strings.NewReader(data)); err == nil {
				t.Fatal("Load accepted invalid fixture")
			}
		})
	}
}

func TestLoadRequiresExplicitItemsArray(t *testing.T) {
	fixture := readFixture(t, "testdata/complete-v1.json")
	tests := map[string]string{
		"omitted": strings.Replace(fixture, ",\n    \"items\": [{\"id\": \"synthetic-network-01\"}]", "", 1),
		"null":    strings.Replace(fixture, `"items": [{"id": "synthetic-network-01"}]`, `"items": null`, 1),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(strings.NewReader(data))
			if err == nil || err.Error() != "networks: items array required" {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadRejectsOversizedFixture(t *testing.T) {
	_, err := Load(strings.NewReader(strings.Repeat(" ", maxSnapshotBytes+1)))
	if err == nil || err.Error() != "decode snapshot: fixture too large" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsDuplicateObjectKeysAtEveryLevel(t *testing.T) {
	fixture := readFixture(t, "testdata/complete-v1.json")
	tests := map[string]string{
		"top level": strings.Replace(fixture, `"schema_version": "v1"`, `"schema_version": "v1", "schema_version": "private-version"`, 1),
		"nested":    strings.Replace(fixture, `"id": "synthetic-network-01"`, `"id": "synthetic-network-01", "id": "private-network-id"`, 1),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(strings.NewReader(data))
			if err == nil || err.Error() != "decode snapshot: duplicate object key" {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadRejectsContradictorySectionState(t *testing.T) {
	fixture := readFixture(t, "testdata/complete-v1.json")
	tests := map[string]string{
		"complete with diagnostics":   strings.Replace(fixture, `"status": "complete",`, `"status": "complete", "diagnostics": ["permission_denied"],`, 1),
		"partial without diagnostics": strings.Replace(fixture, `"status": "complete",`, `"status": "partial",`, 1),
		"unavailable with items": strings.Replace(
			fixture,
			`"status": "complete",`,
			`"status": "unavailable", "diagnostics": ["permission_denied"],`,
			1,
		),
		"duplicate diagnostics": strings.Replace(fixture, `"status": "complete",`, `"status": "partial", "diagnostics": ["permission_denied", "permission_denied"],`, 1),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(strings.NewReader(data)); err == nil {
				t.Fatal("Load accepted contradictory section state")
			}
		})
	}
}

func TestLoadAllowsReferencesMissingFromIncompleteTargetSections(t *testing.T) {
	fixture := readFixture(t, "testdata/complete-v1.json")
	tests := map[string]string{
		"network": strings.Replace(
			strings.Replace(fixture, "\"status\": \"complete\",\n    \"items\": [{\"id\": \"synthetic-network-01\"}]", "\"status\": \"partial\",\n    \"diagnostics\": [\"incomplete_response\"],\n    \"items\": []", 1),
			`"network_id": "synthetic-network-01"`, `"network_id": "private-missing-network"`, 1,
		),
		"device": strings.Replace(
			strings.Replace(fixture, "\"adopted_devices\": {\n    \"status\": \"complete\",", "\"adopted_devices\": {\n    \"status\": \"partial\",\n    \"diagnostics\": [\"incomplete_response\"],", 1),
			`"device_id": "synthetic-device-01"`, `"device_id": "private-missing-device"`, 1,
		),
		"zone": strings.Replace(
			strings.Replace(fixture, "\"firewall_zones\": {\n    \"status\": \"complete\",", "\"firewall_zones\": {\n    \"status\": \"partial\",\n    \"diagnostics\": [\"incomplete_response\"],", 1),
			`"source_zone_id": "synthetic-zone-01"`, `"source_zone_id": "private-missing-zone"`, 1,
		),
		"policy": strings.Replace(
			strings.Replace(fixture, "\"firewall_policies\": {\n    \"status\": \"complete\",", "\"firewall_policies\": {\n    \"status\": \"partial\",\n    \"diagnostics\": [\"incomplete_response\"],", 1),
			`"policy_id": "synthetic-policy-01"`, `"policy_id": "private-missing-policy"`, 1,
		),
		"ACL rule": strings.Replace(
			strings.Replace(fixture, "\"acl_rules\": {\n    \"status\": \"complete\",", "\"acl_rules\": {\n    \"status\": \"unavailable\",\n    \"diagnostics\": [\"endpoint_unsupported\"],", 1),
			`"items": [{"id": "synthetic-rule-01", "network_id": "synthetic-network-01"}]`, `"items": []`, 1,
		),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(strings.NewReader(data)); err != nil {
				t.Fatalf("Load rejected an uncertain reference: %v", err)
			}
		})
	}
}

func TestLoadRejectsReferencesMissingFromCompleteTargetSections(t *testing.T) {
	fixture := readFixture(t, "testdata/complete-v1.json")
	replacements := map[string]string{
		"network":  `"network_id": "private-missing-network"`,
		"device":   `"device_id": "private-missing-device"`,
		"zone":     `"source_zone_id": "private-missing-zone"`,
		"policy":   `"policy_id": "private-missing-policy"`,
		"ACL rule": `"acl_rule_id": "private-missing-rule"`,
	}
	originals := map[string]string{
		"network":  `"network_id": "synthetic-network-01"`,
		"device":   `"device_id": "synthetic-device-01"`,
		"zone":     `"source_zone_id": "synthetic-zone-01"`,
		"policy":   `"policy_id": "synthetic-policy-01"`,
		"ACL rule": `"acl_rule_id": "synthetic-rule-01"`,
	}
	for name, replacement := range replacements {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(strings.NewReader(strings.Replace(fixture, originals[name], replacement, 1))); err == nil {
				t.Fatal("Load accepted a broken reference into a complete section")
			}
		})
	}
}

func TestErrorsDoNotExposeFixtureData(t *testing.T) {
	fixture := readFixture(t, "testdata/complete-v1.json")
	private := []string{"private-schema", "private_status", "private_diagnostic", "private-item-id", "private-target-id", "private_key", "private decoder text"}
	tests := []string{
		strings.Replace(fixture, `"v1"`, `"private-schema"`, 1),
		strings.Replace(fixture, `"complete"`, `"private_status"`, 1),
		strings.Replace(fixture, `"status": "complete",`, `"status": "partial", "diagnostics": ["private_diagnostic"],`, 1),
		strings.Replace(fixture, `"synthetic-network-01"`, `"private-item-id"`, 2),
		strings.Replace(fixture, `"network_id": "synthetic-network-01"`, `"network_id": "private-target-id"`, 1),
		strings.Replace(fixture, `"schema_version": "v1"`, `"schema_version": "v1", "private_key": true`, 1),
		`{"schema_version":"v1","private decoder text"`,
	}
	for i, data := range tests {
		_, err := Load(strings.NewReader(data))
		if err == nil {
			t.Fatalf("case %d unexpectedly loaded", i)
		}
		for _, secret := range private {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("case %d exposed private data in %q", i, err)
			}
		}
	}

	path := "/private/path/controller-export.json"
	if _, err := LoadFile(path); err == nil || strings.Contains(err.Error(), path) {
		t.Fatalf("LoadFile exposed a fixture path: %v", err)
	}
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
