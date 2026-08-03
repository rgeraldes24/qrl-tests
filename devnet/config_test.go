// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package devnet

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultParameters(t *testing.T) {
	address := "Q" + strings.Repeat("a", 128)
	const executionImage = "local/go-qrl:test"
	payload, err := effectiveParameters(address, executionImage, nil)
	require.NoError(t, err)

	var parameters map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &parameters))

	participant := parameters["participants"].([]any)[0].(map[string]any)
	network := parameters["network_params"].(map[string]any)
	prefund := network["prefunded_accounts"].(map[string]any)[address].(map[string]any)
	require.Equal(t, executionImage, participant["el_image"])
	require.Equal(t, consensusImage, participant["cl_image"])
	require.Equal(t, validatorImage, participant["vc_image"])
	require.Equal(t, true, participant["use_remote_signer"])
	require.Equal(t, "clef", participant["remote_signer_type"])
	require.Equal(t, remoteSignerImage, participant["remote_signer_image"])
	require.Equal(t, genesisImage, parameters["qrl_genesis_generator_params"].(map[string]any)["image"])
	require.Equal(t, "1337", network["network_id"])
	require.Equal(t, address, network["withdrawal_address"])
	require.Equal(t, prefundBalance, prefund["balance"])
	require.Regexp(t, `^github\.com/rgeraldes24/qrl-package@[0-9a-f]{40}$`, packageLocator)
}

func TestCustomParameterTokens(t *testing.T) {
	address := "Q" + strings.Repeat("b", 128)
	custom := []byte(`{
		"participants":[{"el_image":"__DEVNET_EXECUTION_IMAGE__","custom":9007199254740993}],
		"network_params":{
			"prefunded_accounts":{"__DEVNET_WALLET_ADDRESS__":{"balance":"1QRL"}},
			"withdrawal_address":"__DEVNET_WALLET_ADDRESS__"
		},
		"untouched":"prefix-__DEVNET_EXECUTION_IMAGE__"
	}`)
	rendered, err := effectiveParameters(address, "registry.example/qrl:test", custom)
	require.NoError(t, err)
	require.Contains(t, rendered, `"custom":9007199254740993`)
	require.Contains(t, rendered, `"el_image":"registry.example/qrl:test"`)
	require.Contains(t, rendered, `"`+address+`":{"balance":"1QRL"}`)
	require.Contains(t, rendered, `"withdrawal_address":"`+address+`"`)
	require.Contains(t, rendered, `"untouched":"prefix-__DEVNET_EXECUTION_IMAGE__"`)
}

func TestInvalidCustomParameters(t *testing.T) {
	address := "Q" + strings.Repeat("c", 128)
	for name, custom := range map[string][]byte{
		"malformed":       []byte(`{`),
		"missing image":   []byte(`{"participants":[{"el_image":"image"}],"network_params":{"prefunded_accounts":{"__DEVNET_WALLET_ADDRESS__":{}}}}`),
		"missing wallet":  []byte(`{"participants":[{"el_image":"__DEVNET_EXECUTION_IMAGE__"}],"network_params":{"prefunded_accounts":{}}}`),
		"escaped image":   []byte(`{"participants":[{"el_image":"__DEVNET_EXECUTION_IMAG\u0045__"}],"network_params":{"prefunded_accounts":{"__DEVNET_WALLET_ADDRESS__":{}}}}`),
		"escaped wallet":  []byte(`{"participants":[{"el_image":"__DEVNET_EXECUTION_IMAGE__"}],"network_params":{"prefunded_accounts":{"__DEVNET_WALLET_ADDR\u0045SS__":{}}}}`),
		"top-level array": []byte(`[]`),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := effectiveParameters(address, "image", custom)
			require.Error(t, err)
		})
	}
}

func TestBuiltInProfiles(t *testing.T) {
	address := "Q" + strings.Repeat("d", 128)
	for _, test := range []struct {
		profile      Profile
		participants int
		validators   int
	}{
		{ProfileSingle, 1, 64},
		{ProfileMulti, 4, 64},
		{ProfileLifecycle, 1, 64},
		{ProfileChaos, 4, 64},
		{ProfileSync, 2, 64},
		{ProfileOperations, 5, 812},
		{ProfileCold, 1, 64},
		{ProfileOptimistic, 2, 64},
		{ProfileExecutionSync, 2, 64},
	} {
		payload, err := effectiveParametersForProfile(address, "image", nil, test.profile)
		require.NoError(t, err)
		var parameters struct {
			Participants []struct {
				ValidatorCount int      `json:"validator_count"`
				ELExtraParams  []string `json:"el_extra_params"`
				CLExtraParams  []string `json:"cl_extra_params"`
				VCExtraParams  []string `json:"vc_extra_params"`
			} `json:"participants"`
			Network struct {
				PreregisteredValidators int `json:"preregistered_validator_count"`
			} `json:"network_params"`
		}
		require.NoError(t, json.Unmarshal([]byte(payload), &parameters))
		require.Len(t, parameters.Participants, test.participants)
		totalValidators := 0
		for _, participant := range parameters.Participants {
			totalValidators += participant.ValidatorCount
		}
		require.Equal(t, test.validators, totalValidators)
		if test.profile == ProfileSync {
			require.Contains(t, parameters.Participants[1].CLExtraParams, "--force-clear-db")
			require.Equal(t, []string{"--enable-doppelganger", "--force-clear-db"}, parameters.Participants[1].VCExtraParams)
		}
		if test.profile == ProfileChaos {
			require.Empty(t, parameters.Participants[0].CLExtraParams)
			require.NotNil(t, parameters.Participants[0].CLExtraParams)
		}
		if test.profile == ProfileCold {
			require.Contains(t, parameters.Participants[0].CLExtraParams, "--slots-per-archive-point=16")
		}
		if test.profile == ProfileOptimistic {
			require.Contains(t, parameters.Participants[1].CLExtraParams, "--startup-optimistic")
		}
		if test.profile == ProfileOperations {
			require.Equal(t, 512, parameters.Network.PreregisteredValidators)
			require.Equal(t, 300, parameters.Participants[4].ValidatorCount)
		}
		if test.profile == ProfileExecutionSync {
			require.Zero(t, parameters.Participants[1].ValidatorCount)
			require.Contains(t, parameters.Participants[1].ELExtraParams, "--nodiscover")
			require.Contains(t, parameters.Participants[1].ELExtraParams, "--bootnodes=")
			require.Contains(t, parameters.Participants[1].CLExtraParams, "--min-sync-peers=0")
		}
	}
}
