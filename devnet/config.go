// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package devnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	packageLocator   = "github.com/rgeraldes24/qrl-package@3892c3d2596403c080424d9e8fc99ff172483fe0"
	defaultNetworkID = "1337"
	prefundBalance   = "2000000QRL"

	executionImagePlaceholder = "__DEVNET_EXECUTION_IMAGE__"
	walletAddressPlaceholder  = "__DEVNET_WALLET_ADDRESS__"

	consensusImage    = "qrledger/qrysm:beacon-chain-8b80fa0c3f5a"
	validatorImage    = "qrledger/qrysm:validator-8b80fa0c3f5a"
	genesisImage      = "qrledger/qrysm:qrl-genesis-generator-360410c72353-8b80fa0c3f5a"
	remoteSignerImage = "local/go-qrl-clef:devnet"

	rpcPortID           = "rpc"
	webSocketPortID     = "ws"
	consensusHTTPPortID = "http"
	metricsPortID       = "metrics"
	graphQLPath         = "/graphql"
)

type parameterShape struct {
	Participants []struct {
		ExecutionImage string `json:"el_image" yaml:"el_image"`
	} `json:"participants" yaml:"participants"`
	Network struct {
		PrefundedAccounts map[string]any `json:"prefunded_accounts" yaml:"prefunded_accounts"`
	} `json:"network_params" yaml:"network_params"`
}

// The qrl-package parameter schema, as far as the built-in profile uses it.
type packageParameters struct {
	Participants  []participant   `json:"participants"`
	NetworkParams networkParams   `json:"network_params"`
	GenesisParams generatorParams `json:"qrl_genesis_generator_params"`
}

type participant struct {
	ELImage           string            `json:"el_image"`
	ELExtraParams     []string          `json:"el_extra_params"`
	CLImage           string            `json:"cl_image"`
	CLExtraParams     []string          `json:"cl_extra_params"`
	VCImage           string            `json:"vc_image"`
	VCExtraParams     []string          `json:"vc_extra_params"`
	UseRemoteSigner   bool              `json:"use_remote_signer"`
	RemoteSignerType  string            `json:"remote_signer_type"`
	RemoteSignerImage string            `json:"remote_signer_image"`
	ValidatorCount    int               `json:"validator_count"`
	ELExtraLabels     map[string]string `json:"el_extra_labels,omitempty"`
	CLExtraLabels     map[string]string `json:"cl_extra_labels,omitempty"`
	VCExtraLabels     map[string]string `json:"vc_extra_labels,omitempty"`
}

type networkParams struct {
	NetworkID               string             `json:"network_id"`
	PreregisteredValidators int                `json:"preregistered_validator_count,omitempty"`
	SecondsPerSlot          int                `json:"seconds_per_slot"`
	SlotsPerEpoch           int                `json:"slots_per_epoch"`
	ExecutionFollowDistance int                `json:"execution_follow_distance"`
	WithdrawabilityDelay    int                `json:"min_validator_withdrawability_delay"`
	ShardCommitteePeriod    int                `json:"shard_committee_period"`
	PrefundedAccounts       map[string]account `json:"prefunded_accounts"`
	WithdrawalAddress       string             `json:"withdrawal_address"`
	LightKDFEnabled         bool               `json:"light_kdf_enabled"`
}

type account struct {
	Balance string `json:"balance"`
}

type generatorParams struct {
	Image string `json:"image"`
}

func effectiveParameters(address, executionImage string, custom []byte) (string, error) {
	return effectiveParametersForProfile(address, executionImage, custom, ProfileSingle)
}

func effectiveParametersForProfile(address, executionImage string, custom []byte, profile Profile) (string, error) {
	if strings.TrimSpace(executionImage) == "" {
		return "", errors.New("execution image is empty")
	}
	if custom != nil {
		return renderCustomParameters(custom, address, executionImage)
	}
	profile, err := normalizeProfile(profile)
	if err != nil {
		return "", err
	}
	spec := profileSpecs[profile]
	participants := make([]participant, len(spec.validatorCounts))
	for index := range participants {
		labels := map[string]string{
			"qrl-tests.participant": strconv.Itoa(index + 1),
			"qrl-tests.partition":   strconv.Itoa(index%2 + 1),
		}
		participants[index] = participant{
			ELImage:           executionImage,
			ELExtraParams:     []string{"--graphql", "--graphql.vhosts=*"},
			CLImage:           consensusImage,
			CLExtraParams:     []string{"--min-sync-peers=0", "--minimum-peers-per-subnet=0"},
			VCImage:           validatorImage,
			VCExtraParams:     []string{},
			UseRemoteSigner:   true,
			RemoteSignerType:  "clef",
			RemoteSignerImage: remoteSignerImage,
			ValidatorCount:    spec.validatorCounts[index],
			ELExtraLabels:     maps.Clone(labels),
			CLExtraLabels:     maps.Clone(labels),
			VCExtraLabels:     maps.Clone(labels),
		}
		if spec.configure != nil {
			spec.configure(index, &participants[index])
		}
	}
	payload, err := json.Marshal(packageParameters{
		Participants: participants,
		NetworkParams: networkParams{
			NetworkID:               defaultNetworkID,
			PreregisteredValidators: spec.preregisteredValidators,
			SecondsPerSlot:          5,
			SlotsPerEpoch:           8,
			ExecutionFollowDistance: 8,
			WithdrawabilityDelay:    2,
			ShardCommitteePeriod:    2,
			PrefundedAccounts:       map[string]account{address: {Balance: prefundBalance}},
			WithdrawalAddress:       address,
			LightKDFEnabled:         true,
		},
		GenesisParams: generatorParams{Image: genesisImage},
	})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

type Profile string

const (
	ProfileSingle        Profile = "single"
	ProfileMulti         Profile = "multi"
	ProfileLifecycle     Profile = "lifecycle"
	ProfileChaos         Profile = "chaos"
	ProfileSync          Profile = "sync"
	ProfileOperations    Profile = "operations"
	ProfileCold          Profile = "cold"
	ProfileOptimistic    Profile = "optimistic"
	ProfileExecutionSync Profile = "execution-sync"
)

type profileSpec struct {
	validatorCounts         []int
	preregisteredValidators int
	configure               func(int, *participant)
}

var profileSpecs = map[Profile]profileSpec{
	ProfileSingle:    {validatorCounts: []int{64}},
	ProfileMulti:     {validatorCounts: []int{16, 16, 16, 16}},
	ProfileLifecycle: {validatorCounts: []int{64}},
	ProfileChaos: {
		validatorCounts: []int{16, 16, 16, 16},
		configure: func(_ int, participant *participant) {
			participant.CLExtraParams = []string{}
		},
	},
	ProfileSync: {
		validatorCounts: []int{32, 32},
		configure: func(index int, participant *participant) {
			if index == 1 {
				participant.CLExtraParams = append(participant.CLExtraParams, "--force-clear-db")
				participant.VCExtraParams = []string{"--enable-doppelganger", "--force-clear-db"}
			}
		},
	},
	ProfileOperations: {
		validatorCounts:         []int{128, 128, 128, 128, 300},
		preregisteredValidators: 512,
	},
	ProfileCold: {
		validatorCounts: []int{64},
		configure: func(_ int, participant *participant) {
			participant.CLExtraParams = append(participant.CLExtraParams, "--slots-per-archive-point=16")
		},
	},
	ProfileOptimistic: {
		validatorCounts: []int{32, 32},
		configure: func(index int, participant *participant) {
			if index == 1 {
				participant.CLExtraParams = append(participant.CLExtraParams, "--startup-optimistic")
			}
		},
	},
	ProfileExecutionSync: {
		validatorCounts: []int{64, 0},
		configure: func(index int, participant *participant) {
			if index != 1 {
				return
			}
			participant.ELExtraParams = append(participant.ELExtraParams, "--nodiscover", "--bootnodes=")
		},
	},
}

func normalizeProfile(profile Profile) (Profile, error) {
	if profile == "" {
		return ProfileSingle, nil
	}
	switch profile {
	case ProfileSingle, ProfileMulti, ProfileLifecycle, ProfileChaos, ProfileSync, ProfileOperations,
		ProfileCold, ProfileOptimistic, ProfileExecutionSync:
		return profile, nil
	default:
		return "", fmt.Errorf("unknown development-network profile %q", profile)
	}
}

func renderCustomParameters(payload []byte, address, executionImage string) (string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(payload, &document); err != nil {
		return "", errors.New("parameters file must contain one YAML mapping")
	}
	shape, err := decodeParameterShape(&document)
	if err != nil {
		return "", err
	}
	if len(shape.Participants) == 0 || shape.Participants[0].ExecutionImage != executionImagePlaceholder {
		return "", fmt.Errorf(
			"first participant el_image must be %q",
			executionImagePlaceholder,
		)
	}
	if _, ok := shape.Network.PrefundedAccounts[walletAddressPlaceholder]; !ok {
		return "", fmt.Errorf(
			"network_params.prefunded_accounts must contain %q",
			walletAddressPlaceholder,
		)
	}

	replaceParameterTokens(&document, map[string]string{
		executionImagePlaceholder: executionImage,
		walletAddressPlaceholder:  address,
	})
	rendered, err := yaml.Marshal(&document)
	if err != nil {
		return "", fmt.Errorf("encode rendered parameters: %w", err)
	}

	var renderedDocument yaml.Node
	if err := yaml.Unmarshal(rendered, &renderedDocument); err != nil {
		return "", errors.New("rendered parameters must contain one YAML mapping")
	}
	renderedShape, err := decodeParameterShape(&renderedDocument)
	if err != nil {
		return "", errors.New("rendered parameters must contain one YAML mapping")
	}
	if len(renderedShape.Participants) == 0 ||
		renderedShape.Participants[0].ExecutionImage != executionImage {
		return "", errors.New("execution-image token was not replaced")
	}
	if _, ok := renderedShape.Network.PrefundedAccounts[address]; !ok {
		return "", errors.New("wallet-address token was not replaced")
	}
	return string(rendered), nil
}

func decodeParameterShape(document *yaml.Node) (parameterShape, error) {
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return parameterShape{}, errors.New("parameters file must contain one YAML mapping")
	}
	var shape parameterShape
	if err := document.Decode(&shape); err != nil {
		return parameterShape{}, errors.New("parameters file must contain one YAML mapping")
	}
	return shape, nil
}

func replaceParameterTokens(node *yaml.Node, replacements map[string]string) {
	if node.Kind == yaml.ScalarNode {
		if replacement, ok := replacements[node.Value]; ok {
			node.Value = replacement
		}
	}
	for _, child := range node.Content {
		replaceParameterTokens(child, replacements)
	}
}
