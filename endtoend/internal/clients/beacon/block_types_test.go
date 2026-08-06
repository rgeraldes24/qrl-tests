// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package beacon

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignedBlockJSON(t *testing.T) {
	input := []byte(`{
		"signature":"block-signature",
		"message":{
			"slot":"12","proposer_index":"3","parent_root":"parent","state_root":"state",
			"body":{
				"attestations":[{"data":{"slot":"11","index":"2","beacon_block_root":"root","source":{"epoch":"1","root":"source"},"target":{"epoch":"2","root":"target"}}}],
				"execution_payload":{"block_number":"9","gas_limit":"10","gas_used":"8","withdrawals":[{"index":"4","validator_index":"5","amount":"6"}]},
				"execution_data":{"deposit_count":"7"},
				"deposits":[{"data":{"amount":"32"}}],
				"voluntary_exits":[{"message":{"epoch":"2","validator_index":"6"}}],
				"proposer_slashings":[{"signed_header_1":{"message":{"slot":"12","proposer_index":"3"},"signature":"header-signature"},"signed_header_2":{"message":{"slot":"12","proposer_index":"3"},"signature":"header-signature"}}],
				"attester_slashings":[{"attestation_1":{"attesting_indices":["7","8"],"data":{"slot":"11","index":"2","source":{"epoch":"1"},"target":{"epoch":"2"}}},"attestation_2":{"attesting_indices":["9"],"data":{"slot":"11","index":"2","source":{"epoch":"1"},"target":{"epoch":"2"}}}}]
			}
		}
	}`)

	var block SignedBlock
	require.NoError(t, json.Unmarshal(input, &block))
	require.Equal(t, uint64(12), block.Message.Slot)
	require.Equal(t, uint64(3), block.Message.ProposerIndex)
	require.Equal(t, uint64(9), block.Message.Body.ExecutionPayload.BlockNumber)
	require.Equal(t, uint64(6), block.Message.Body.ExecutionPayload.Withdrawals[0].Amount)
	require.Equal(t, uint64(7), block.Message.Body.ExecutionData.DepositCount)
	require.Equal(t, uint64(11), block.Message.Body.Attestations[0].Data.Slot)
	require.Equal(t, uint64(32), block.Message.Body.Deposits[0].Data.Amount)
	require.Equal(t, uint64(6), block.Message.Body.VoluntaryExits[0].Message.ValidatorIndex)
	require.Equal(t, uint64(3), block.Message.Body.ProposerSlashings[0].Header1.Message.ProposerIndex)
	require.Equal(t, []uint64{7, 8}, block.Message.Body.AttesterSlashings[0].Attestation1.AttestingIndices)
}

func TestSignedBlockJSONRejectsInvalidNestedNumber(t *testing.T) {
	input := []byte(`{"message":{"slot":"1","proposer_index":"2","body":{"execution_payload":{"block_number":"3","gas_limit":"4","gas_used":"5","withdrawals":[{"index":"6","validator_index":"7","amount":"invalid"}]},"execution_data":{"deposit_count":"8"}}}}`)

	var block SignedBlock
	require.Error(t, json.Unmarshal(input, &block))
}

func TestIndexedAttestationJSON(t *testing.T) {
	want := IndexedAttestation{
		AttestingIndices: []uint64{7, 8},
		Data: AttestationData{
			Slot:            9,
			CommitteeIndex:  10,
			BeaconBlockRoot: "0x01",
			Source:          Checkpoint{Epoch: 11, Root: "0x02"},
			Target:          Checkpoint{Epoch: 12, Root: "0x03"},
		},
		Signatures: []string{"0x04"},
	}
	payload, err := json.Marshal(want)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"attesting_indices":["7","8"],
		"data":{
			"slot":"9",
			"index":"10",
			"beacon_block_root":"0x01",
			"source":{"epoch":"11","root":"0x02"},
			"target":{"epoch":"12","root":"0x03"}
		},
		"signatures":["0x04"]
	}`, string(payload))

	var got IndexedAttestation
	require.NoError(t, json.Unmarshal(payload, &got))
	require.Equal(t, want, got)
}
