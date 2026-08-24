package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const SchemaVersion = "v1"

type Status string

const (
	StatusComplete    Status = "complete"
	StatusPartial     Status = "partial"
	StatusUnavailable Status = "unavailable"
)

type DiagnosticCode string

const (
	DiagnosticSourceUnreachable   DiagnosticCode = "source_unreachable"
	DiagnosticEndpointUnsupported DiagnosticCode = "endpoint_unsupported"
	DiagnosticPermissionDenied    DiagnosticCode = "permission_denied"
	DiagnosticIncompleteResponse  DiagnosticCode = "incomplete_response"
	DiagnosticInvalidSourceData   DiagnosticCode = "invalid_source_data"
)

type Section[T any] struct {
	Status      Status           `json:"status"`
	Diagnostics []DiagnosticCode `json:"diagnostics,omitempty"`
	Items       []T              `json:"items"`
}

type Snapshot struct {
	SchemaVersion    string                   `json:"schema_version"`
	Networks         Section[Network]         `json:"networks"`
	FirewallZones    Section[FirewallZone]    `json:"firewall_zones"`
	AdoptedDevices   Section[AdoptedDevice]   `json:"adopted_devices"`
	ActiveClients    Section[ActiveClient]    `json:"active_clients"`
	WiFiBroadcasts   Section[WiFiBroadcast]   `json:"wifi_broadcasts"`
	FirewallPolicies Section[FirewallPolicy]  `json:"firewall_policies"`
	ACLRules         Section[ACLRule]         `json:"acl_rules"`
	PolicyReferences Section[PolicyReference] `json:"policy_references"`
}

type Network struct {
	ID string `json:"id"`
}

type FirewallZone struct {
	ID         string   `json:"id"`
	NetworkIDs []string `json:"network_ids"`
}

type AdoptedDevice struct {
	ID        string `json:"id"`
	NetworkID string `json:"network_id"`
}

type ActiveClient struct {
	ID        string `json:"id"`
	NetworkID string `json:"network_id"`
	DeviceID  string `json:"device_id,omitempty"`
}

type WiFiBroadcast struct {
	ID        string `json:"id"`
	NetworkID string `json:"network_id"`
}

type FirewallPolicy struct {
	ID                string `json:"id"`
	SourceZoneID      string `json:"source_zone_id"`
	DestinationZoneID string `json:"destination_zone_id"`
}

type ACLRule struct {
	ID        string `json:"id"`
	NetworkID string `json:"network_id"`
}

type PolicyReference struct {
	ID        string `json:"id"`
	PolicyID  string `json:"policy_id"`
	ACLRuleID string `json:"acl_rule_id"`
}

func LoadFile(path string) (*Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

func Load(r io.Reader) (*Snapshot, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("decode snapshot: trailing JSON data")
	}
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (s Snapshot) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema version %q", s.SchemaVersion)
	}

	networks, err := validateSection("networks", s.Networks, func(item Network) string { return item.ID })
	if err != nil {
		return err
	}
	zones, err := validateSection("firewall_zones", s.FirewallZones, func(item FirewallZone) string { return item.ID })
	if err != nil {
		return err
	}
	devices, err := validateSection("adopted_devices", s.AdoptedDevices, func(item AdoptedDevice) string { return item.ID })
	if err != nil {
		return err
	}
	if _, err = validateSection("active_clients", s.ActiveClients, func(item ActiveClient) string { return item.ID }); err != nil {
		return err
	}
	if _, err = validateSection("wifi_broadcasts", s.WiFiBroadcasts, func(item WiFiBroadcast) string { return item.ID }); err != nil {
		return err
	}
	policies, err := validateSection("firewall_policies", s.FirewallPolicies, func(item FirewallPolicy) string { return item.ID })
	if err != nil {
		return err
	}
	rules, err := validateSection("acl_rules", s.ACLRules, func(item ACLRule) string { return item.ID })
	if err != nil {
		return err
	}
	if _, err = validateSection("policy_references", s.PolicyReferences, func(item PolicyReference) string { return item.ID }); err != nil {
		return err
	}

	for _, zone := range s.FirewallZones.Items {
		for _, id := range zone.NetworkIDs {
			if err := requireReference("firewall_zones", zone.ID, "network", id, networks); err != nil {
				return err
			}
		}
	}
	for _, device := range s.AdoptedDevices.Items {
		if err := requireReference("adopted_devices", device.ID, "network", device.NetworkID, networks); err != nil {
			return err
		}
	}
	for _, client := range s.ActiveClients.Items {
		if err := requireReference("active_clients", client.ID, "network", client.NetworkID, networks); err != nil {
			return err
		}
		if client.DeviceID != "" {
			if err := requireReference("active_clients", client.ID, "device", client.DeviceID, devices); err != nil {
				return err
			}
		}
	}
	for _, broadcast := range s.WiFiBroadcasts.Items {
		if err := requireReference("wifi_broadcasts", broadcast.ID, "network", broadcast.NetworkID, networks); err != nil {
			return err
		}
	}
	for _, policy := range s.FirewallPolicies.Items {
		if err := requireReference("firewall_policies", policy.ID, "source zone", policy.SourceZoneID, zones); err != nil {
			return err
		}
		if err := requireReference("firewall_policies", policy.ID, "destination zone", policy.DestinationZoneID, zones); err != nil {
			return err
		}
	}
	for _, rule := range s.ACLRules.Items {
		if err := requireReference("acl_rules", rule.ID, "network", rule.NetworkID, networks); err != nil {
			return err
		}
	}
	for _, reference := range s.PolicyReferences.Items {
		if err := requireReference("policy_references", reference.ID, "policy", reference.PolicyID, policies); err != nil {
			return err
		}
		if err := requireReference("policy_references", reference.ID, "ACL rule", reference.ACLRuleID, rules); err != nil {
			return err
		}
	}
	return nil
}

func validateSection[T any](name string, section Section[T], id func(T) string) (map[string]struct{}, error) {
	if section.Status != StatusComplete && section.Status != StatusPartial && section.Status != StatusUnavailable {
		return nil, fmt.Errorf("%s: invalid status %q", name, section.Status)
	}
	if section.Status != StatusComplete && len(section.Diagnostics) == 0 {
		return nil, fmt.Errorf("%s: %s section requires diagnostics", name, section.Status)
	}
	if section.Status == StatusUnavailable && len(section.Items) != 0 {
		return nil, fmt.Errorf("%s: unavailable section must not contain items", name)
	}
	for _, code := range section.Diagnostics {
		if !validDiagnostic(code) {
			return nil, fmt.Errorf("%s: invalid diagnostic code %q", name, code)
		}
	}

	ids := make(map[string]struct{}, len(section.Items))
	for _, item := range section.Items {
		itemID := id(item)
		if itemID == "" {
			return nil, fmt.Errorf("%s: item has empty ID", name)
		}
		if _, exists := ids[itemID]; exists {
			return nil, fmt.Errorf("%s: duplicate ID %q", name, itemID)
		}
		ids[itemID] = struct{}{}
	}
	return ids, nil
}

func validDiagnostic(code DiagnosticCode) bool {
	switch code {
	case DiagnosticSourceUnreachable, DiagnosticEndpointUnsupported, DiagnosticPermissionDenied, DiagnosticIncompleteResponse, DiagnosticInvalidSourceData:
		return true
	default:
		return false
	}
}

func requireReference(section, itemID, kind, targetID string, targets map[string]struct{}) error {
	if _, ok := targets[targetID]; !ok {
		return fmt.Errorf("%s: item %q has broken %s reference %q", section, itemID, kind, targetID)
	}
	return nil
}
