// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package devnet

import (
	"fmt"
	"strings"
)

type Backend string

const (
	BackendDocker     Backend = "docker"
	BackendKubernetes Backend = "kubernetes"
)

func ParseBackend(value string) (Backend, error) {
	backend := Backend(strings.TrimSpace(value))
	if backend == "" {
		return BackendDocker, nil
	}
	switch backend {
	case BackendDocker, BackendKubernetes:
		return backend, nil
	default:
		return "", fmt.Errorf("unsupported Kurtosis backend %q", value)
	}
}

type Images struct {
	Execution string
	Clef      string
	Consensus string
	Validator string
	Genesis   string
}

func (images Images) withDefaults() Images {
	if images.Clef == "" {
		images.Clef = DefaultClefImage
	}
	if images.Consensus == "" {
		images.Consensus = DefaultConsensusImage
	}
	if images.Validator == "" {
		images.Validator = DefaultValidatorImage
	}
	if images.Genesis == "" {
		images.Genesis = DefaultGenesisImage
	}
	return images
}

func (images Images) validate(backend Backend) error {
	for _, item := range []struct {
		name, image string
	}{
		{"execution", images.Execution},
		{"Clef", images.Clef},
		{"consensus", images.Consensus},
		{"validator", images.Validator},
		{"genesis", images.Genesis},
	} {
		if strings.TrimSpace(item.image) == "" {
			return fmt.Errorf("%s image is empty", item.name)
		}
		if backend == BackendKubernetes && strings.HasPrefix(item.image, "local/") {
			return fmt.Errorf("%s image %q is not available to Kubernetes; use a registry image", item.name, item.image)
		}
	}
	return nil
}
