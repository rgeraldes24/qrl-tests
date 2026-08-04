// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package apicoverage

import "testing"

func TestValidate(t *testing.T) {
	scenarios := map[Scenario]string{"live": "live scenario"}
	if err := Validate(map[string]Entry{"method": Live(Behavior, "live")}, scenarios); err != nil {
		t.Fatal(err)
	}
	if err := Validate(map[string]Entry{"method": Excluded("disabled")}, scenarios); err != nil {
		t.Fatal(err)
	}
	if err := Validate(map[string]Entry{"method": Live(Behavior, "missing")}, scenarios); err == nil {
		t.Fatal("expected unknown scenario error")
	}
}

func TestInventoryDigest(t *testing.T) {
	entries := map[string]Entry{"second": {}, "first": {}}
	const want = "dbea9325179efe46ea2add94f7b6b745ca983fabb208dc6d34aa064623d7ee23"
	if got := InventoryDigest(entries); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
