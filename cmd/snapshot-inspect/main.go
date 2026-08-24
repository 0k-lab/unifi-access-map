package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/0k-lab/unifi-access-map/internal/snapshot"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: snapshot-inspect <fixture.json>")
		return 2
	}

	value, err := snapshot.LoadFile(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "snapshot-inspect: invalid fixture")
		return 1
	}

	fmt.Fprintf(stdout, "schema_version: %s\n", value.SchemaVersion)
	printSection(stdout, "networks", value.Networks.Status, len(value.Networks.Items), value.Networks.Diagnostics)
	printSection(stdout, "firewall_zones", value.FirewallZones.Status, len(value.FirewallZones.Items), value.FirewallZones.Diagnostics)
	printSection(stdout, "adopted_devices", value.AdoptedDevices.Status, len(value.AdoptedDevices.Items), value.AdoptedDevices.Diagnostics)
	printSection(stdout, "active_clients", value.ActiveClients.Status, len(value.ActiveClients.Items), value.ActiveClients.Diagnostics)
	printSection(stdout, "wifi_broadcasts", value.WiFiBroadcasts.Status, len(value.WiFiBroadcasts.Items), value.WiFiBroadcasts.Diagnostics)
	printSection(stdout, "firewall_policies", value.FirewallPolicies.Status, len(value.FirewallPolicies.Items), value.FirewallPolicies.Diagnostics)
	printSection(stdout, "acl_rules", value.ACLRules.Status, len(value.ACLRules.Items), value.ACLRules.Diagnostics)
	printSection(stdout, "policy_references", value.PolicyReferences.Status, len(value.PolicyReferences.Items), value.PolicyReferences.Diagnostics)
	return 0
}

func printSection(w io.Writer, name string, status snapshot.Status, count int, diagnostics []snapshot.DiagnosticCode) {
	codes := make([]string, len(diagnostics))
	for i, code := range diagnostics {
		codes[i] = string(code)
	}
	if len(codes) == 0 {
		codes = append(codes, "-")
	}
	fmt.Fprintf(w, "%s: %s count=%d diagnostics=%s\n", name, status, count, strings.Join(codes, ","))
}
