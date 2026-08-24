package snapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	SchemaVersion    = "v1"
	maxSnapshotBytes = 1 << 20
	maxJSONDepth     = 64
)

var errDuplicateObjectKey = errors.New("duplicate object key")

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
		return nil, errors.New("open snapshot fixture: failed")
	}
	defer f.Close()
	return Load(f)
}

func Load(r io.Reader) (*Snapshot, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSnapshotBytes+1))
	if err != nil {
		return nil, errors.New("read snapshot fixture: failed")
	}
	if len(data) > maxSnapshotBytes {
		return nil, errors.New("decode snapshot: fixture too large")
	}
	if err := rejectDuplicateObjectKeys(data); err != nil {
		if errors.Is(err, errDuplicateObjectKey) {
			return nil, errors.New("decode snapshot: duplicate object key")
		}
		return nil, errors.New("decode snapshot: invalid JSON")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, errors.New("decode snapshot: invalid structure")
	}
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (s Snapshot) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return errors.New("snapshot: unsupported schema version")
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
			if err := requireReference("firewall_zones", "network", id, networks, s.Networks.Status); err != nil {
				return err
			}
		}
	}
	for _, device := range s.AdoptedDevices.Items {
		if err := requireReference("adopted_devices", "network", device.NetworkID, networks, s.Networks.Status); err != nil {
			return err
		}
	}
	for _, client := range s.ActiveClients.Items {
		if err := requireReference("active_clients", "network", client.NetworkID, networks, s.Networks.Status); err != nil {
			return err
		}
		if client.DeviceID != "" {
			if err := requireReference("active_clients", "device", client.DeviceID, devices, s.AdoptedDevices.Status); err != nil {
				return err
			}
		}
	}
	for _, broadcast := range s.WiFiBroadcasts.Items {
		if err := requireReference("wifi_broadcasts", "network", broadcast.NetworkID, networks, s.Networks.Status); err != nil {
			return err
		}
	}
	for _, policy := range s.FirewallPolicies.Items {
		if err := requireReference("firewall_policies", "source zone", policy.SourceZoneID, zones, s.FirewallZones.Status); err != nil {
			return err
		}
		if err := requireReference("firewall_policies", "destination zone", policy.DestinationZoneID, zones, s.FirewallZones.Status); err != nil {
			return err
		}
	}
	for _, rule := range s.ACLRules.Items {
		if err := requireReference("acl_rules", "network", rule.NetworkID, networks, s.Networks.Status); err != nil {
			return err
		}
	}
	for _, reference := range s.PolicyReferences.Items {
		if err := requireReference("policy_references", "policy", reference.PolicyID, policies, s.FirewallPolicies.Status); err != nil {
			return err
		}
		if err := requireReference("policy_references", "ACL rule", reference.ACLRuleID, rules, s.ACLRules.Status); err != nil {
			return err
		}
	}
	return nil
}

func validateSection[T any](name string, section Section[T], id func(T) string) (map[string]struct{}, error) {
	if section.Status != StatusComplete && section.Status != StatusPartial && section.Status != StatusUnavailable {
		return nil, fmt.Errorf("%s: invalid status", name)
	}
	if section.Items == nil {
		return nil, fmt.Errorf("%s: items array required", name)
	}
	if section.Status != StatusComplete && len(section.Diagnostics) == 0 {
		return nil, fmt.Errorf("%s: diagnostics required", name)
	}
	if section.Status == StatusComplete && len(section.Diagnostics) != 0 {
		return nil, fmt.Errorf("%s: diagnostics forbidden for section state", name)
	}
	if section.Status == StatusUnavailable && len(section.Items) != 0 {
		return nil, fmt.Errorf("%s: items forbidden for section state", name)
	}
	diagnostics := make(map[DiagnosticCode]struct{}, len(section.Diagnostics))
	for _, code := range section.Diagnostics {
		if !validDiagnostic(code) {
			return nil, fmt.Errorf("%s: invalid diagnostic code", name)
		}
		if _, exists := diagnostics[code]; exists {
			return nil, fmt.Errorf("%s: duplicate diagnostic code", name)
		}
		diagnostics[code] = struct{}{}
	}

	ids := make(map[string]struct{}, len(section.Items))
	for _, item := range section.Items {
		itemID := id(item)
		if itemID == "" {
			return nil, fmt.Errorf("%s: item has empty ID", name)
		}
		if _, exists := ids[itemID]; exists {
			return nil, fmt.Errorf("%s: duplicate item ID", name)
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

func requireReference(section, kind, targetID string, targets map[string]struct{}, targetStatus Status) error {
	if _, ok := targets[targetID]; !ok && targetStatus == StatusComplete {
		return fmt.Errorf("%s: broken %s reference", section, kind)
	}
	return nil
}

func rejectDuplicateObjectKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := walkJSONValue(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON data")
	}
	return nil
}

func walkJSONValue(decoder *json.Decoder, depth int) error {
	if depth > maxJSONDepth {
		return errors.New("JSON nesting too deep")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return errors.New("invalid object key")
			}
			if _, exists := keys[name]; exists {
				return errDuplicateObjectKey
			}
			keys[name] = struct{}{}
			if err := walkJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := walkJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}
