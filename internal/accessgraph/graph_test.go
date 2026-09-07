package accessgraph

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/0k-lab/unifi-access-map/internal/snapshot"
)

func fixture(t *testing.T, name string) snapshot.Snapshot {
	t.Helper()
	s, err := snapshot.LoadFile("../snapshot/testdata/" + name + "-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return *s
}

func project(t *testing.T, s snapshot.Snapshot) Graph {
	t.Helper()
	g, err := Project(s)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestCompleteFixture(t *testing.T) {
	g := project(t, fixture(t, "complete"))
	network := Node{Network, "synthetic-network-01"}
	zone1 := Node{FirewallZone, "synthetic-zone-01"}
	zone2 := Node{FirewallZone, "synthetic-zone-02"}
	device := Node{AdoptedDevice, "synthetic-device-01"}
	client := Node{ActiveClient, "synthetic-client-01"}
	wifi := Node{WiFiBroadcast, "synthetic-broadcast-01"}
	policy := Node{FirewallPolicy, "synthetic-policy-01"}
	rule := Node{ACLRule, "synthetic-rule-01"}
	want := Graph{
		Nodes: []Node{rule, client, device, policy, zone1, zone2, network, wifi},
		Edges: []Edge{
			{ACLToNetwork, rule, network},
			{ClientToDevice, client, device},
			{ClientToNetwork, client, network},
			{DeviceToNetwork, device, network},
			{PolicyToACL, policy, rule},
			{PolicyToDestinationZone, policy, zone2},
			{PolicyToSourceZone, policy, zone1},
			{WiFiToNetwork, wifi, network},
			{ZoneToNetwork, zone1, network},
		},
		IncompleteSections: []string{},
	}
	if !reflect.DeepEqual(g, want) {
		t.Fatalf("got %+v; want %+v", g, want)
	}
}

func TestDegradedFixture(t *testing.T) {
	g := project(t, fixture(t, "degraded"))
	client := Node{ActiveClient, "synthetic-client-01"}
	network := Node{Network, "synthetic-network-01"}
	want := Graph{
		Nodes:              []Node{client, network},
		Edges:              []Edge{{ClientToNetwork, client, network}},
		IncompleteSections: []string{"active_clients", "wifi_broadcasts"},
	}
	if !reflect.DeepEqual(g, want) {
		t.Fatalf("got %+v; want %+v", g, want)
	}
}

func encoded(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDeterministicOrderingAndNoMutation(t *testing.T) {
	s := fixture(t, "complete")
	// Give every section multiple items, including IDs shared across node kinds.
	s.Networks.Items = append(s.Networks.Items, snapshot.Network{ID: "synthetic-shared"})
	s.FirewallZones.Items[0].NetworkIDs = append(s.FirewallZones.Items[0].NetworkIDs, "synthetic-shared", "synthetic-shared")
	s.AdoptedDevices.Items = append(s.AdoptedDevices.Items, snapshot.AdoptedDevice{ID: "synthetic-shared", NetworkID: "synthetic-shared"})
	s.ActiveClients.Items = append(s.ActiveClients.Items, snapshot.ActiveClient{ID: "synthetic-shared", NetworkID: "synthetic-shared", DeviceID: "synthetic-shared"})
	s.WiFiBroadcasts.Items = append(s.WiFiBroadcasts.Items, snapshot.WiFiBroadcast{ID: "synthetic-shared", NetworkID: "synthetic-shared"})
	s.FirewallPolicies.Items = append(s.FirewallPolicies.Items, snapshot.FirewallPolicy{ID: "synthetic-shared", SourceZoneID: "synthetic-zone-01", DestinationZoneID: "synthetic-zone-01"})
	s.ACLRules.Items = append(s.ACLRules.Items, snapshot.ACLRule{ID: "synthetic-shared", NetworkID: "synthetic-shared"})
	s.PolicyReferences.Items = append(s.PolicyReferences.Items, snapshot.PolicyReference{ID: "synthetic-reference-02", PolicyID: "synthetic-policy-01", ACLRuleID: "synthetic-rule-01"}, snapshot.PolicyReference{ID: "synthetic-reference-03", PolicyID: "synthetic-shared", ACLRuleID: "synthetic-shared"})
	before := encoded(t, s)
	g := project(t, s)
	if !bytes.Equal(before, encoded(t, s)) {
		t.Fatal("projection mutated snapshot")
	}
	if len(g.Nodes) != 14 || len(g.Edges) != 18 {
		t.Fatalf("lost typed identities or retained duplicate edges: %+v", g)
	}
	want := encoded(t, g)
	for i := 0; i < 20; i++ {
		slices.Reverse(s.Networks.Items)
		slices.Reverse(s.FirewallZones.Items)
		for j := range s.FirewallZones.Items {
			slices.Reverse(s.FirewallZones.Items[j].NetworkIDs)
		}
		slices.Reverse(s.AdoptedDevices.Items)
		slices.Reverse(s.ActiveClients.Items)
		slices.Reverse(s.WiFiBroadcasts.Items)
		slices.Reverse(s.FirewallPolicies.Items)
		slices.Reverse(s.ACLRules.Items)
		slices.Reverse(s.PolicyReferences.Items)
		if !bytes.Equal(want, encoded(t, project(t, s))) {
			t.Fatal("projection bytes depend on input or map ordering")
		}
	}
}

func emptySection[T any](section *snapshot.Section[T], status snapshot.Status) {
	section.Status = status
	section.Items = []T{}
	if status != snapshot.StatusComplete {
		section.Diagnostics = []snapshot.DiagnosticCode{snapshot.DiagnosticIncompleteResponse}
	}
}

func TestMissingEndpoints(t *testing.T) {
	tests := []struct {
		name   string
		kind   NodeKind
		remove func(*snapshot.Snapshot, snapshot.Status)
	}{
		{"networks", Network, func(s *snapshot.Snapshot, status snapshot.Status) { emptySection(&s.Networks, status) }},
		{"firewall_zones", FirewallZone, func(s *snapshot.Snapshot, status snapshot.Status) { emptySection(&s.FirewallZones, status) }},
		{"adopted_devices", AdoptedDevice, func(s *snapshot.Snapshot, status snapshot.Status) { emptySection(&s.AdoptedDevices, status) }},
		{"firewall_policies", FirewallPolicy, func(s *snapshot.Snapshot, status snapshot.Status) { emptySection(&s.FirewallPolicies, status) }},
		{"acl_rules", ACLRule, func(s *snapshot.Snapshot, status snapshot.Status) { emptySection(&s.ACLRules, status) }},
	}
	for _, tt := range tests {
		for _, status := range []snapshot.Status{snapshot.StatusComplete, snapshot.StatusPartial, snapshot.StatusUnavailable} {
			t.Run(tt.name+"/"+string(status), func(t *testing.T) {
				s := fixture(t, "complete")
				want := project(t, s)
				tt.remove(&s, status)
				g, err := Project(s)
				if status == snapshot.StatusComplete {
					if err == nil || !reflect.DeepEqual(g, Graph{}) {
						t.Fatal("expected validation error and no graph")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				want.Nodes = slices.DeleteFunc(want.Nodes, func(n Node) bool { return n.Kind == tt.kind })
				want.Edges = slices.DeleteFunc(want.Edges, func(e Edge) bool { return e.From.Kind == tt.kind || e.To.Kind == tt.kind })
				want.IncompleteSections = []string{tt.name}
				if !reflect.DeepEqual(g, want) {
					t.Fatalf("got %+v; want %+v", g, want)
				}
			})
		}
	}
}

func TestPartialSectionsRetainKnownRelationships(t *testing.T) {
	s := fixture(t, "complete")
	s.FirewallZones.Status = snapshot.StatusPartial
	s.FirewallZones.Diagnostics = []snapshot.DiagnosticCode{snapshot.DiagnosticIncompleteResponse}
	s.FirewallZones.Items = s.FirewallZones.Items[:1]
	s.PolicyReferences.Status = snapshot.StatusPartial
	s.PolicyReferences.Diagnostics = []snapshot.DiagnosticCode{snapshot.DiagnosticIncompleteResponse}
	g := project(t, s)
	if len(g.Edges) != 8 || !slices.Equal(g.IncompleteSections, []string{"firewall_zones", "policy_references"}) {
		t.Fatalf("unexpected partial graph: %+v", g)
	}
	for _, edge := range g.Edges {
		if edge.Kind == PolicyToDestinationZone {
			t.Fatal("invented missing destination zone")
		}
	}
}

func TestRejectInvalidSnapshot(t *testing.T) {
	if g, err := Project(snapshot.Snapshot{}); err == nil || !reflect.DeepEqual(g, Graph{}) {
		t.Fatal("expected validation error and no graph")
	}
}
