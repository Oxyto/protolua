package protoflux

import "testing"

func TestResolveKnownNodes(t *testing.T) {
	for _, raw := range []string{"Write", "ProtoFlux:Write", "Actions.Write", "https://wiki.resonite.com/ProtoFlux%3AWrite"} {
		resolved := Resolve(raw)
		if !resolved.Known {
			t.Fatalf("%q should resolve as a known node: %#v", raw, resolved)
		}
		if resolved.Canonical != "ProtoFlux:Write" {
			t.Fatalf("%q canonical = %q, want ProtoFlux:Write", raw, resolved.Canonical)
		}
		if resolved.Path == "" {
			t.Fatalf("%q resolved to an empty path", raw)
		}
	}
}

func TestResolveUnknownNodeStaysAddressable(t *testing.T) {
	resolved := Resolve("Community.Custom.Node")
	if resolved.Known {
		t.Fatalf("custom node should stay unknown: %#v", resolved)
	}
	if resolved.Path != "Community.Custom.Node" {
		t.Fatalf("path = %q, want Community.Custom.Node", resolved.Path)
	}
	if resolved.Canonical != "ProtoFlux:Node" {
		t.Fatalf("canonical = %q, want ProtoFlux:Node", resolved.Canonical)
	}
	if resolved.Category != "Community.Custom" {
		t.Fatalf("category = %q, want Community.Custom", resolved.Category)
	}
}

func TestSearchFindsAliases(t *testing.T) {
	results := Search("Variables.Dynamic.WriteOrCreate", 5)
	if len(results) == 0 {
		t.Fatalf("expected at least one result")
	}
	if results[0].Name != "WriteOrCreateDynamicVariable" {
		t.Fatalf("first result = %s, want WriteOrCreateDynamicVariable", results[0].Name)
	}
}
