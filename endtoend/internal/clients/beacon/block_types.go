// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package beacon

// SignedBlock is a beacon block with numeric REST fields decoded into their
// protocol values. Encoded roots, bitfields, keys, and signatures remain in
// their wire form so callers can validate them independently.
type SignedBlock struct {
	Message   BlockMessage `json:"message"`
	Signature string       `json:"signature"`
}

type BlockMessage struct {
	Slot          uint64    `json:"slot,string"`
	ProposerIndex uint64    `json:"proposer_index,string"`
	ParentRoot    string    `json:"parent_root"`
	StateRoot     string    `json:"state_root"`
	Body          BlockBody `json:"body"`
}

type BlockBody struct {
	Graffiti          string                `json:"graffiti"`
	RandaoReveal      string                `json:"randao_reveal"`
	Attestations      []Attestation         `json:"attestations"`
	ExecutionPayload  ExecutionPayload      `json:"execution_payload"`
	ExecutionData     ExecutionDataVote     `json:"execution_data"`
	SyncAggregate     SyncAggregate         `json:"sync_aggregate"`
	Deposits          []DepositOperation    `json:"deposits"`
	VoluntaryExits    []SignedVoluntaryExit `json:"voluntary_exits"`
	ProposerSlashings []ProposerSlashing    `json:"proposer_slashings"`
	AttesterSlashings []AttesterSlashing    `json:"attester_slashings"`
}

type SyncAggregate struct {
	Bits       string   `json:"sync_committee_bits"`
	Signatures []string `json:"sync_committee_signatures"`
}

type DepositOperation struct {
	Data Deposit `json:"data"`
}

type SignedVoluntaryExit struct {
	Message   VoluntaryExit `json:"message"`
	Signature string        `json:"signature"`
}

type VoluntaryExit struct {
	Epoch          uint64 `json:"epoch,string"`
	ValidatorIndex uint64 `json:"validator_index,string"`
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
	Slot          uint64 `json:"slot,string"`
	ProposerIndex uint64 `json:"proposer_index,string"`
	ParentRoot    string `json:"parent_root"`
	StateRoot     string `json:"state_root"`
	BodyRoot      string `json:"body_root"`
}

type SignedBeaconBlockHeader struct {
	Message   BeaconBlockHeader `json:"message"`
	Signature string            `json:"signature"`
}

type BlockHeader struct {
	Root      string                  `json:"root"`
	Canonical bool                    `json:"canonical"`
	Header    SignedBeaconBlockHeader `json:"header"`
}

type AttestationData struct {
	Slot            uint64     `json:"slot,string"`
	CommitteeIndex  uint64     `json:"index,string"`
	BeaconBlockRoot string     `json:"beacon_block_root"`
	Source          Checkpoint `json:"source"`
	Target          Checkpoint `json:"target"`
}

type Attestation struct {
	AggregationBits string          `json:"aggregation_bits"`
	Data            AttestationData `json:"data"`
	Signatures      []string        `json:"signatures"`
}

type IndexedAttestation struct {
	AttestingIndices []uint64
	Data             AttestationData `json:"data"`
	Signatures       []string        `json:"signatures"`
}
