// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensus

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

type SignedBlock struct {
	Message   BlockMessage `json:"message"`
	Signature string       `json:"signature"`
}

type BlockMessage struct {
	Slot          string    `json:"slot"`
	ProposerIndex string    `json:"proposer_index"`
	ParentRoot    string    `json:"parent_root"`
	StateRoot     string    `json:"state_root"`
	Body          BlockBody `json:"body"`
}

type BlockBody struct {
	Graffiti          string                `json:"graffiti"`
	RandaoReveal      string                `json:"randao_reveal"`
	Attestations      []Attestation         `json:"attestations"`
	ExecutionPayload  ExecutionPayloadData  `json:"execution_payload"`
	ExecutionData     ExecutionData         `json:"execution_data"`
	SyncAggregate     SyncAggregate         `json:"sync_aggregate"`
	Deposits          []DepositOperation    `json:"deposits"`
	VoluntaryExits    []SignedVoluntaryExit `json:"voluntary_exits"`
	ProposerSlashings []ProposerSlashing    `json:"proposer_slashings"`
	AttesterSlashings []AttesterSlashing    `json:"attester_slashings"`
}

type ExecutionPayloadData struct {
	ParentHash   string           `json:"parent_hash"`
	FeeRecipient string           `json:"fee_recipient"`
	BlockNumber  string           `json:"block_number"`
	GasLimit     string           `json:"gas_limit"`
	GasUsed      string           `json:"gas_used"`
	BlockHash    string           `json:"block_hash"`
	Transactions []string         `json:"transactions"`
	Withdrawals  []WithdrawalData `json:"withdrawals"`
}

type WithdrawalData struct {
	Index          string `json:"index"`
	ValidatorIndex string `json:"validator_index"`
	Address        string `json:"address"`
	Amount         string `json:"amount"`
}

type ExecutionData struct {
	DepositRoot  string `json:"deposit_root"`
	DepositCount string `json:"deposit_count"`
	BlockHash    string `json:"block_hash"`
}

type SyncAggregate struct {
	Bits       string   `json:"sync_committee_bits"`
	Signatures []string `json:"sync_committee_signatures"`
}

type DepositOperation struct {
	Data DepositData `json:"data"`
}

type DepositData struct {
	PublicKey             string `json:"pubkey"`
	WithdrawalCredentials string `json:"withdrawal_credentials"`
	Amount                string `json:"amount"`
	Signature             string `json:"signature"`
}

type SignedVoluntaryExit struct {
	Message   VoluntaryExit `json:"message"`
	Signature string        `json:"signature"`
}

type VoluntaryExit struct {
	Epoch          string `json:"epoch"`
	ValidatorIndex string `json:"validator_index"`
}

type ProposerSlashing struct {
	Header1 SignedBeaconBlockHeader `json:"signed_header_1"`
	Header2 SignedBeaconBlockHeader `json:"signed_header_2"`
}

type AttesterSlashing struct {
	Attestation1 IndexedAttestation `json:"attestation_1"`
	Attestation2 IndexedAttestation `json:"attestation_2"`
}

type BeaconBlockHeader struct {
	Slot          string `json:"slot"`
	ProposerIndex string `json:"proposer_index"`
	ParentRoot    string `json:"parent_root"`
	StateRoot     string `json:"state_root"`
	BodyRoot      string `json:"body_root"`
}

type SignedBeaconBlockHeader struct {
	Message   BeaconBlockHeader `json:"message"`
	Signature string            `json:"signature"`
}

type BlockHeader struct {
	Root      string
	Canonical bool
	Header    SignedBeaconBlockHeader
}

type CheckpointData struct {
	Epoch string `json:"epoch"`
	Root  string `json:"root"`
}

type AttestationData struct {
	Slot            string         `json:"slot"`
	CommitteeIndex  string         `json:"index"`
	BeaconBlockRoot string         `json:"beacon_block_root"`
	Source          CheckpointData `json:"source"`
	Target          CheckpointData `json:"target"`
}

type Attestation struct {
	AggregationBits string          `json:"aggregation_bits"`
	Data            AttestationData `json:"data"`
	Signatures      []string        `json:"signatures"`
}

type IndexedAttestation struct {
	AttestingIndices []string        `json:"attesting_indices"`
	Data             AttestationData `json:"data"`
	Signatures       []string        `json:"signatures"`
}

func (client *Client) Block(ctx context.Context, blockID string) (SignedBlock, error) {
	var response dataResponse[SignedBlock]
	if err := client.get(ctx, "/qrl/v1/beacon/blocks/"+url.PathEscape(blockID), &response); err != nil {
		return SignedBlock{}, err
	}
	return response.Data, nil
}

func (client *Client) BlockHeader(ctx context.Context, blockID string) (BlockHeader, error) {
	var response struct {
		Data struct {
			Root      string                  `json:"root"`
			Canonical bool                    `json:"canonical"`
			Header    SignedBeaconBlockHeader `json:"header"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/headers/"+url.PathEscape(blockID), &response); err != nil {
		return BlockHeader{}, err
	}
	return BlockHeader{
		Root: response.Data.Root, Canonical: response.Data.Canonical, Header: response.Data.Header,
	}, nil
}

func (client *Client) BlockGraffiti(ctx context.Context, blockID string) (string, error) {
	block, err := client.Block(ctx, blockID)
	if err != nil {
		return "", err
	}
	return block.Message.Body.Graffiti, nil
}

func (client *Client) BlockExecutionPayload(ctx context.Context, blockID string) (ExecutionPayload, error) {
	block, err := client.Block(ctx, blockID)
	if err != nil {
		return ExecutionPayload{}, err
	}
	return block.ExecutionPayload()
}

func (block SignedBlock) ExecutionPayload() (ExecutionPayload, error) {
	raw := block.Message.Body.ExecutionPayload
	blockNumber, err := decimal("execution block number", raw.BlockNumber)
	if err != nil {
		return ExecutionPayload{}, err
	}
	gasLimit, err := decimal("execution gas limit", raw.GasLimit)
	if err != nil {
		return ExecutionPayload{}, err
	}
	gasUsed, err := decimal("execution gas used", raw.GasUsed)
	if err != nil {
		return ExecutionPayload{}, err
	}
	withdrawals := make([]Withdrawal, len(raw.Withdrawals))
	for index, item := range raw.Withdrawals {
		withdrawalIndex, err := decimal("withdrawal index", item.Index)
		if err != nil {
			return ExecutionPayload{}, err
		}
		validatorIndex, err := decimal("withdrawal validator index", item.ValidatorIndex)
		if err != nil {
			return ExecutionPayload{}, err
		}
		amount, err := decimal("withdrawal amount", item.Amount)
		if err != nil {
			return ExecutionPayload{}, err
		}
		withdrawals[index] = Withdrawal{withdrawalIndex, validatorIndex, item.Address, amount}
	}
	return ExecutionPayload{
		ParentHash: raw.ParentHash, FeeRecipient: raw.FeeRecipient,
		BlockNumber: blockNumber, GasLimit: gasLimit, GasUsed: gasUsed,
		BlockHash: raw.BlockHash, Transactions: raw.Transactions, Withdrawals: withdrawals,
	}, nil
}

func (client *Client) BlockConsensusData(ctx context.Context, blockID string) (BlockConsensusData, error) {
	block, err := client.Block(ctx, blockID)
	if err != nil {
		return BlockConsensusData{}, err
	}
	return block.ConsensusData()
}

func (block SignedBlock) ConsensusData() (BlockConsensusData, error) {
	message := block.Message
	slot, err := decimal("block slot", message.Slot)
	if err != nil {
		return BlockConsensusData{}, err
	}
	proposer, err := decimal("block proposer index", message.ProposerIndex)
	if err != nil {
		return BlockConsensusData{}, err
	}
	depositCount, err := decimal("execution data deposit count", message.Body.ExecutionData.DepositCount)
	if err != nil {
		return BlockConsensusData{}, err
	}
	return BlockConsensusData{
		Slot:          slot,
		ProposerIndex: proposer,
		ParentRoot:    message.ParentRoot,
		StateRoot:     message.StateRoot,
		ExecutionData: ExecutionDataVote{
			DepositRoot:  message.Body.ExecutionData.DepositRoot,
			DepositCount: depositCount,
			BlockHash:    message.Body.ExecutionData.BlockHash,
		},
		SyncCommitteeBits:       message.Body.SyncAggregate.Bits,
		SyncCommitteeSignatures: message.Body.SyncAggregate.Signatures,
	}, nil
}

func (client *Client) ValidatorAssignments(ctx context.Context, epoch uint64) ([]ValidatorAssignment, error) {
	var response struct {
		Epoch       string `json:"epoch"`
		Assignments []struct {
			BeaconCommittee []string `json:"beaconCommittees"`
			CommitteeIndex  string   `json:"committeeIndex"`
			AttesterSlot    string   `json:"attesterSlot"`
			ProposerSlots   []string `json:"proposerSlots"`
			ValidatorIndex  string   `json:"validatorIndex"`
		} `json:"assignments"`
		NextPageToken string `json:"nextPageToken"`
		TotalSize     int    `json:"totalSize"`
	}
	path := "/qrl/v1alpha1/validators/assignments?epoch=" + strconv.FormatUint(epoch, 10) + "&page_size=250"
	if err := client.get(ctx, path, &response); err != nil {
		return nil, err
	}
	assignments := make([]ValidatorAssignment, len(response.Assignments))
	for index, raw := range response.Assignments {
		validatorIndex, err := decimal("assignment validator index", raw.ValidatorIndex)
		if err != nil {
			return nil, err
		}
		committeeIndex, err := decimal("assignment committee index", raw.CommitteeIndex)
		if err != nil {
			return nil, err
		}
		attesterSlot, err := decimal("assignment attester slot", raw.AttesterSlot)
		if err != nil {
			return nil, err
		}
		assignment := ValidatorAssignment{
			ValidatorIndex: validatorIndex,
			CommitteeIndex: committeeIndex,
			AttesterSlot:   attesterSlot,
		}
		for _, value := range raw.ProposerSlots {
			parsed, err := decimal("assignment proposer slot", value)
			if err != nil {
				return nil, err
			}
			assignment.ProposerSlots = append(assignment.ProposerSlots, parsed)
		}
		for _, value := range raw.BeaconCommittee {
			parsed, err := decimal("assignment committee member", value)
			if err != nil {
				return nil, err
			}
			assignment.BeaconCommittee = append(assignment.BeaconCommittee, parsed)
		}
		assignments[index] = assignment
	}
	if response.TotalSize != 0 && response.TotalSize != len(assignments) {
		return nil, fmt.Errorf("validator assignments returned %d of %d entries", len(assignments), response.TotalSize)
	}
	if response.NextPageToken != "" {
		return nil, errors.New("validator assignments exceed one response page")
	}
	return assignments, nil
}

func (client *Client) BlockOperations(ctx context.Context, blockID string) (BlockOperations, error) {
	block, err := client.Block(ctx, blockID)
	if err != nil {
		return BlockOperations{}, err
	}
	return block.Operations()
}

func (block SignedBlock) Operations() (BlockOperations, error) {
	body := block.Message.Body
	result := BlockOperations{}
	for _, item := range body.Deposits {
		amount, err := decimal("deposit amount", item.Data.Amount)
		if err != nil {
			return BlockOperations{}, err
		}
		result.Deposits = append(result.Deposits, Deposit{
			PublicKey:             item.Data.PublicKey,
			WithdrawalCredentials: item.Data.WithdrawalCredentials,
			Amount:                amount,
			Signature:             item.Data.Signature,
		})
	}
	for _, item := range body.VoluntaryExits {
		value, err := decimal("voluntary exit validator index", item.Message.ValidatorIndex)
		if err != nil {
			return BlockOperations{}, err
		}
		result.VoluntaryExits = append(result.VoluntaryExits, value)
	}
	for _, item := range body.ProposerSlashings {
		value, err := decimal("proposer slashing validator index", item.Header1.Message.ProposerIndex)
		if err != nil {
			return BlockOperations{}, err
		}
		result.ProposerSlashings = append(result.ProposerSlashings, value)
	}
	for _, item := range body.AttesterSlashings {
		for _, raw := range item.Attestation1.AttestingIndices {
			value, err := decimal("attester slashing validator index", raw)
			if err != nil {
				return BlockOperations{}, err
			}
			result.AttesterSlashings = append(result.AttesterSlashings, value)
		}
	}
	for _, item := range body.ExecutionPayload.Withdrawals {
		index, err := decimal("withdrawal index", item.Index)
		if err != nil {
			return BlockOperations{}, err
		}
		validatorIndex, err := decimal("withdrawal validator index", item.ValidatorIndex)
		if err != nil {
			return BlockOperations{}, err
		}
		amount, err := decimal("withdrawal amount", item.Amount)
		if err != nil {
			return BlockOperations{}, err
		}
		result.Withdrawals = append(result.Withdrawals, Withdrawal{index, validatorIndex, item.Address, amount})
	}
	return result, nil
}
