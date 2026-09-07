// Package accessgraph projects normalized snapshots into reference-only graphs.
package accessgraph

import (
	"cmp"
	"slices"

	"github.com/0k-lab/unifi-access-map/internal/snapshot"
)

type NodeKind string

const (
	Network        NodeKind = "network"
	FirewallZone   NodeKind = "firewall_zone"
	AdoptedDevice  NodeKind = "adopted_device"
	ActiveClient   NodeKind = "active_client"
	WiFiBroadcast  NodeKind = "wifi_broadcast"
	FirewallPolicy NodeKind = "firewall_policy"
	ACLRule        NodeKind = "acl_rule"
)

// Node identity includes Kind because snapshot IDs are unique only per section.
type Node struct {
	Kind NodeKind `json:"kind"`
	ID   string   `json:"id"`
}

type EdgeKind string

const (
	ZoneToNetwork           EdgeKind = "zone_to_network"
	DeviceToNetwork         EdgeKind = "device_to_network"
	ClientToNetwork         EdgeKind = "client_to_network"
	WiFiToNetwork           EdgeKind = "wifi_to_network"
	ClientToDevice          EdgeKind = "client_to_device"
	PolicyToSourceZone      EdgeKind = "policy_to_source_zone"
	PolicyToDestinationZone EdgeKind = "policy_to_destination_zone"
	ACLToNetwork            EdgeKind = "acl_to_network"
	PolicyToACL             EdgeKind = "policy_to_acl"
)

type Edge struct {
	Kind EdgeKind `json:"kind"`
	From Node     `json:"from"`
	To   Node     `json:"to"`
}

type Graph struct {
	Nodes              []Node   `json:"nodes"`
	Edges              []Edge   `json:"edges"`
	IncompleteSections []string `json:"incomplete_sections"`
}

// Project validates s, then returns a graph without modifying the snapshot.
// Nodes sort by (kind, ID), edges by (kind, from kind/ID, to kind/ID), and
// incomplete section names lexically. Repeated relationships collapse to one edge.
func Project(s snapshot.Snapshot) (Graph, error) {
	if err := s.Validate(); err != nil {
		return Graph{}, err
	}
	g := Graph{Nodes: []Node{}, Edges: []Edge{}, IncompleteSections: []string{}}
	nodes := make(map[Node]bool)
	edges := make(map[Edge]bool)
	addNode := func(kind NodeKind, id string) {
		node := Node{kind, id}
		nodes[node] = true
		g.Nodes = append(g.Nodes, node)
	}
	addEdge := func(kind EdgeKind, fromKind NodeKind, fromID string, toKind NodeKind, toID string) {
		edges[Edge{kind, Node{fromKind, fromID}, Node{toKind, toID}}] = true
	}
	for _, item := range s.Networks.Items {
		addNode(Network, item.ID)
	}
	for _, item := range s.FirewallZones.Items {
		addNode(FirewallZone, item.ID)
		for _, id := range item.NetworkIDs {
			addEdge(ZoneToNetwork, FirewallZone, item.ID, Network, id)
		}
	}
	for _, item := range s.AdoptedDevices.Items {
		addNode(AdoptedDevice, item.ID)
		addEdge(DeviceToNetwork, AdoptedDevice, item.ID, Network, item.NetworkID)
	}
	for _, item := range s.ActiveClients.Items {
		addNode(ActiveClient, item.ID)
		addEdge(ClientToNetwork, ActiveClient, item.ID, Network, item.NetworkID)
		if item.DeviceID != "" {
			addEdge(ClientToDevice, ActiveClient, item.ID, AdoptedDevice, item.DeviceID)
		}
	}
	for _, item := range s.WiFiBroadcasts.Items {
		addNode(WiFiBroadcast, item.ID)
		addEdge(WiFiToNetwork, WiFiBroadcast, item.ID, Network, item.NetworkID)
	}
	for _, item := range s.FirewallPolicies.Items {
		addNode(FirewallPolicy, item.ID)
		addEdge(PolicyToSourceZone, FirewallPolicy, item.ID, FirewallZone, item.SourceZoneID)
		addEdge(PolicyToDestinationZone, FirewallPolicy, item.ID, FirewallZone, item.DestinationZoneID)
	}
	for _, item := range s.ACLRules.Items {
		addNode(ACLRule, item.ID)
		addEdge(ACLToNetwork, ACLRule, item.ID, Network, item.NetworkID)
	}
	for _, item := range s.PolicyReferences.Items {
		addEdge(PolicyToACL, FirewallPolicy, item.PolicyID, ACLRule, item.ACLRuleID)
	}
	for edge := range edges {
		if nodes[edge.From] && nodes[edge.To] {
			g.Edges = append(g.Edges, edge)
		}
	}
	for name, status := range map[string]snapshot.Status{
		"networks": s.Networks.Status, "firewall_zones": s.FirewallZones.Status,
		"adopted_devices": s.AdoptedDevices.Status, "active_clients": s.ActiveClients.Status,
		"wifi_broadcasts": s.WiFiBroadcasts.Status, "firewall_policies": s.FirewallPolicies.Status,
		"acl_rules": s.ACLRules.Status, "policy_references": s.PolicyReferences.Status,
	} {
		if status != snapshot.StatusComplete {
			g.IncompleteSections = append(g.IncompleteSections, name)
		}
	}
	slices.SortFunc(g.Nodes, compareNodes)
	slices.SortFunc(g.Edges, func(a, b Edge) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), compareNodes(a.From, b.From), compareNodes(a.To, b.To))
	})
	slices.Sort(g.IncompleteSections)
	return g, nil
}

func compareNodes(a, b Node) int {
	return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.ID, b.ID))
}
