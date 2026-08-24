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
