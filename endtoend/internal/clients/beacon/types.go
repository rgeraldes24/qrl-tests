// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package beacon

type SyncStatus struct {
	HeadSlot   uint64 `json:"head_slot,string"`
	Syncing    bool   `json:"is_syncing"`
	Optimistic bool   `json:"is_optimistic"`
	ELOffline  bool   `json:"el_offline"`
}

type Head struct {
	Slot uint64 `json:"slot,string"`
	Root string `json:"root"`
}

type Checkpoint struct {
	Epoch uint64 `json:"epoch,string"`
	Root  string `json:"root"`
}

type ValidatorLiveness struct {
	Index  uint64 `json:"index,string"`
	IsLive bool   `json:"is_live"`
}

type ValidatorParticipation struct {
	PreviousActive uint64 `json:"previousEpochActiveShor,string"`
	PreviousTarget uint64 `json:"previousEpochTargetAttestingShor,string"`
	PreviousHead   uint64 `json:"previousEpochHeadAttestingShor,string"`
}

type ExecutionPayload struct {
	ParentHash   string       `json:"parent_hash"`
	FeeRecipient string       `json:"fee_recipient"`
	BlockNumber  uint64       `json:"block_number,string"`
	GasLimit     uint64       `json:"gas_limit,string"`
	GasUsed      uint64       `json:"gas_used,string"`
	BlockHash    string       `json:"block_hash"`
	Transactions []string     `json:"transactions"`
	Withdrawals  []Withdrawal `json:"withdrawals"`
}

type Withdrawal struct {
	Index          uint64 `json:"index,string"`
	ValidatorIndex uint64 `json:"validator_index,string"`
	Address        string `json:"address"`
	Amount         uint64 `json:"amount,string"`
}

type Genesis struct {
	Time           uint64 `json:"genesis_time,string"`
	ValidatorsRoot string `json:"genesis_validators_root"`
	ForkVersion    string `json:"genesis_fork_version"`
}

type Fork struct {
	PreviousVersion string `json:"previous_version"`
	CurrentVersion  string `json:"current_version"`
	Epoch           uint64 `json:"epoch,string"`
}

type Validator struct {
	Index             uint64
	Balance           uint64
	Status            string
	PublicKey         string
	Withdrawal        string
	EffectiveBalance  uint64
	Slashed           bool
	ActivationEpoch   uint64
	ExitEpoch         uint64
	WithdrawableEpoch uint64
}

type DepositContract struct {
	ChainID uint64 `json:"chain_id,string"`
	Address string `json:"address"`
}

type BlockOperations struct {
	Deposits          []Deposit
	VoluntaryExits    []uint64
	ProposerSlashings []uint64
	AttesterSlashings []uint64
	Withdrawals       []Withdrawal
}

type Deposit struct {
	PublicKey             string `json:"pubkey"`
	WithdrawalCredentials string `json:"withdrawal_credentials"`
	Amount                uint64 `json:"amount,string"`
	Signature             string `json:"signature"`
}

type ExecutionDataVote struct {
	DepositRoot  string `json:"deposit_root"`
	DepositCount uint64 `json:"deposit_count,string"`
	BlockHash    string `json:"block_hash"`
}

type ValidatorAssignment struct {
	ValidatorIndex  uint64
	CommitteeIndex  uint64
	AttesterSlot    uint64
	ProposerSlots   []uint64
	BeaconCommittee []uint64
}
