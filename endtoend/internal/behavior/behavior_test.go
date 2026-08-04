// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package behavior

import "testing"

func TestName(t *testing.T) {
	if got, want := Name("network:finality"), "behavior:network:finality"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
