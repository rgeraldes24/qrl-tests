// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensus

type SyncStatus struct {
	HeadSlot   uint64
	Syncing    bool
	Optimistic bool
	ELOffline  bool
}

type Head struct {
	Slot uint64
	Root string
}

type Checkpoint struct {
	Epoch uint64
	Root  string
}

type ValidatorLiveness struct {
	Index  uint64
	IsLive bool
}

type ValidatorParticipation struct {
	PreviousActive uint64
	PreviousTarget uint64
	PreviousHead   uint64
}

type ExecutionPayload struct {
	ParentHash   string
	FeeRecipient string
	BlockNumber  uint64
	GasLimit     uint64
	GasUsed      uint64
	BlockHash    string
	Transactions []string
	Withdrawals  []Withdrawal
}

type Withdrawal struct {
	Index          uint64
	ValidatorIndex uint64
	Address        string
	Amount         uint64
}

type Genesis struct {
	Time           uint64
	ValidatorsRoot string
	ForkVersion    string
}

type Fork struct {
	PreviousVersion string
	CurrentVersion  string
	Epoch           uint64
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
	ChainID uint64
	Address string
}

type BlockOperations struct {
	Deposits          []Deposit
	VoluntaryExits    []uint64
	ProposerSlashings []uint64
	AttesterSlashings []uint64
	Withdrawals       []Withdrawal
}

type Deposit struct {
	PublicKey             string
	WithdrawalCredentials string
	Amount                uint64
	Signature             string
}

type ExecutionDataVote struct {
	DepositRoot  string
	DepositCount uint64
	BlockHash    string
}

type BlockConsensusData struct {
	Slot                    uint64
	ProposerIndex           uint64
	ParentRoot              string
	StateRoot               string
	ExecutionData           ExecutionDataVote
	SyncCommitteeBits       string
	SyncCommitteeSignatures []string
}

type ValidatorAssignment struct {
	ValidatorIndex  uint64
	CommitteeIndex  uint64
	AttesterSlot    uint64
	ProposerSlots   []uint64
	BeaconCommittee []uint64
}
