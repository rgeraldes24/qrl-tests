// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package behavior defines the stable IDs used to connect executable specs to
// the scenario coverage catalog.
package behavior

type ID string

func Name(id ID) string {
	return "behavior:" + string(id)
}
