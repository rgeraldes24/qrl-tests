// Package consensus provides the beacon REST operations used by live suites.
package consensus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type Client struct {
	baseURL *url.URL
	http    *http.Client
}

type responseError struct {
	method     string
	path       string
	status     string
	statusCode int
	body       string
}

func (err *responseError) Error() string {
	return fmt.Sprintf("%s %s returned %s: %s", err.method, err.path, err.status, err.body)
}

func IsNotFound(err error) bool {
	var responseErr *responseError
	return errors.As(err, &responseErr) && responseErr.statusCode == http.StatusNotFound
}

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

func New(endpoint string) (*Client, error) {
	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse consensus endpoint: %w", err)
	}
	return &Client{baseURL: baseURL, http: http.DefaultClient}, nil
}

func (client *Client) Health(ctx context.Context) error {
	return client.get(ctx, "/qrl/v1/node/health", nil)
}

func (client *Client) Syncing(ctx context.Context) (SyncStatus, error) {
	var response struct {
		Data struct {
			HeadSlot   string `json:"head_slot"`
			Syncing    bool   `json:"is_syncing"`
			Optimistic bool   `json:"is_optimistic"`
			ELOffline  bool   `json:"el_offline"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/node/syncing", &response); err != nil {
		return SyncStatus{}, err
	}
	headSlot, err := decimal("head slot", response.Data.HeadSlot)
	if err != nil {
		return SyncStatus{}, err
	}
	return SyncStatus{
		HeadSlot:   headSlot,
		Syncing:    response.Data.Syncing,
		Optimistic: response.Data.Optimistic,
		ELOffline:  response.Data.ELOffline,
	}, nil
}

func (client *Client) HeadSlot(ctx context.Context) (uint64, error) {
	head, err := client.Head(ctx)
	return head.Slot, err
}

func (client *Client) Head(ctx context.Context) (Head, error) {
	return client.Header(ctx, "head")
}

func (client *Client) Header(ctx context.Context, blockID string) (Head, error) {
	var response struct {
		Data struct {
			Root   string `json:"root"`
			Header struct {
				Message struct {
					Slot string `json:"slot"`
				} `json:"message"`
			} `json:"header"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/headers/"+url.PathEscape(blockID), &response); err != nil {
		return Head{}, err
	}
	slot, err := decimal("head slot", response.Data.Header.Message.Slot)
	if err != nil {
		return Head{}, err
	}
	return Head{Slot: slot, Root: response.Data.Root}, nil
}

func (client *Client) ValidatorParticipation(ctx context.Context) (ValidatorParticipation, error) {
	var response struct {
		Participation struct {
			PreviousActive string `json:"previousEpochActiveShor"`
			PreviousTarget string `json:"previousEpochTargetAttestingShor"`
			PreviousHead   string `json:"previousEpochHeadAttestingShor"`
		} `json:"participation"`
	}
	if err := client.get(ctx, "/qrl/v1alpha1/validators/participation", &response); err != nil {
		return ValidatorParticipation{}, err
	}
	active, err := decimal("previous active balance", response.Participation.PreviousActive)
	if err != nil {
		return ValidatorParticipation{}, err
	}
	target, err := decimal("previous target-attesting balance", response.Participation.PreviousTarget)
	if err != nil {
		return ValidatorParticipation{}, err
	}
	head, err := decimal("previous head-attesting balance", response.Participation.PreviousHead)
	if err != nil {
		return ValidatorParticipation{}, err
	}
	return ValidatorParticipation{PreviousActive: active, PreviousTarget: target, PreviousHead: head}, nil
}

func (client *Client) FinalizedEpoch(ctx context.Context) (uint64, error) {
	checkpoint, err := client.FinalizedCheckpoint(ctx)
	return checkpoint.Epoch, err
}

func (client *Client) FinalizedCheckpoint(ctx context.Context) (Checkpoint, error) {
	var response struct {
		Data struct {
			Finalized struct {
				Epoch string `json:"epoch"`
				Root  string `json:"root"`
			} `json:"finalized"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/finality_checkpoints", &response); err != nil {
		return Checkpoint{}, err
	}
	epoch, err := decimal("finalized epoch", response.Data.Finalized.Epoch)
	if err != nil {
		return Checkpoint{}, err
	}
	return Checkpoint{Epoch: epoch, Root: response.Data.Finalized.Root}, nil
}

func (client *Client) Genesis(ctx context.Context) (Genesis, error) {
	var response struct {
		Data struct {
			Time           string `json:"genesis_time"`
			ValidatorsRoot string `json:"genesis_validators_root"`
			ForkVersion    string `json:"genesis_fork_version"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/genesis", &response); err != nil {
		return Genesis{}, err
	}
	genesisTime, err := decimal("genesis time", response.Data.Time)
	if err != nil {
		return Genesis{}, err
	}
	return Genesis{genesisTime, response.Data.ValidatorsRoot, response.Data.ForkVersion}, nil
}

func (client *Client) Fork(ctx context.Context) (Fork, error) {
	var response struct {
		Data struct {
			PreviousVersion string `json:"previous_version"`
			CurrentVersion  string `json:"current_version"`
			Epoch           string `json:"epoch"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/fork", &response); err != nil {
		return Fork{}, err
	}
	epoch, err := decimal("fork epoch", response.Data.Epoch)
	if err != nil {
		return Fork{}, err
	}
	return Fork{response.Data.PreviousVersion, response.Data.CurrentVersion, epoch}, nil
}

func (client *Client) Validator(ctx context.Context, validatorID string) (Validator, error) {
	var response struct {
		Data struct {
			Index     string `json:"index"`
			Balance   string `json:"balance"`
			Status    string `json:"status"`
			Validator struct {
				PublicKey         string `json:"pubkey"`
				Withdrawal        string `json:"withdrawal_credentials"`
				EffectiveBalance  string `json:"effective_balance"`
				Slashed           bool   `json:"slashed"`
				ActivationEpoch   string `json:"activation_epoch"`
				ExitEpoch         string `json:"exit_epoch"`
				WithdrawableEpoch string `json:"withdrawable_epoch"`
			} `json:"validator"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/validators/"+url.PathEscape(validatorID), &response); err != nil {
		return Validator{}, err
	}
	index, err := decimal("validator index", response.Data.Index)
	if err != nil {
		return Validator{}, err
	}
	balance, err := decimal("validator balance", response.Data.Balance)
	if err != nil {
		return Validator{}, err
	}
	effectiveBalance, err := decimal("validator effective balance", response.Data.Validator.EffectiveBalance)
	if err != nil {
		return Validator{}, err
	}
	activationEpoch, err := decimal("validator activation epoch", response.Data.Validator.ActivationEpoch)
	if err != nil {
		return Validator{}, err
	}
	exitEpoch, err := decimal("validator exit epoch", response.Data.Validator.ExitEpoch)
	if err != nil {
		return Validator{}, err
	}
	withdrawableEpoch, err := decimal("validator withdrawable epoch", response.Data.Validator.WithdrawableEpoch)
	if err != nil {
		return Validator{}, err
	}
	return Validator{
		Index: index, Balance: balance, Status: response.Data.Status,
		PublicKey: response.Data.Validator.PublicKey, Withdrawal: response.Data.Validator.Withdrawal,
		EffectiveBalance: effectiveBalance, Slashed: response.Data.Validator.Slashed,
		ActivationEpoch: activationEpoch, ExitEpoch: exitEpoch, WithdrawableEpoch: withdrawableEpoch,
	}, nil
}

func (client *Client) DepositContract(ctx context.Context) (DepositContract, error) {
	var response struct {
		Data struct {
			ChainID string `json:"chain_id"`
			Address string `json:"address"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/config/deposit_contract", &response); err != nil {
		return DepositContract{}, err
	}
	chainID, err := decimal("deposit contract chain ID", response.Data.ChainID)
	if err != nil {
		return DepositContract{}, err
	}
	return DepositContract{ChainID: chainID, Address: response.Data.Address}, nil
}

func (client *Client) ActiveValidatorCount(ctx context.Context) (int, error) {
	indices, err := client.ActiveValidatorIndices(ctx)
	return len(indices), err
}

func (client *Client) ActiveValidatorIndices(ctx context.Context) ([]uint64, error) {
	var response struct {
		Data []struct {
			Index string `json:"index"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/validators?status=active", &response); err != nil {
		return nil, err
	}
	indices := make([]uint64, len(response.Data))
	for index, validator := range response.Data {
		value, err := decimal("validator index", validator.Index)
		if err != nil {
			return nil, err
		}
		indices[index] = value
	}
	return indices, nil
}

func (client *Client) Validators(ctx context.Context, status string) ([]Validator, error) {
	path := "/qrl/v1/beacon/states/head/validators"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	var response struct {
		Data []struct {
			Index     string `json:"index"`
			Balance   string `json:"balance"`
			Status    string `json:"status"`
			Validator struct {
				PublicKey         string `json:"pubkey"`
				Withdrawal        string `json:"withdrawal_credentials"`
				EffectiveBalance  string `json:"effective_balance"`
				Slashed           bool   `json:"slashed"`
				ActivationEpoch   string `json:"activation_epoch"`
				ExitEpoch         string `json:"exit_epoch"`
				WithdrawableEpoch string `json:"withdrawable_epoch"`
			} `json:"validator"`
		} `json:"data"`
	}
	if err := client.get(ctx, path, &response); err != nil {
		return nil, err
	}
	validators := make([]Validator, len(response.Data))
	for index, item := range response.Data {
		validator, err := parseValidator(
			item.Index,
			item.Balance,
			item.Status,
			item.Validator.PublicKey,
			item.Validator.Withdrawal,
			item.Validator.EffectiveBalance,
			item.Validator.Slashed,
			item.Validator.ActivationEpoch,
			item.Validator.ExitEpoch,
			item.Validator.WithdrawableEpoch,
		)
		if err != nil {
			return nil, err
		}
		validators[index] = validator
	}
	return validators, nil
}

func (client *Client) BlockAttestationCount(ctx context.Context, blockID string) (int, error) {
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/blocks/"+url.PathEscape(blockID)+"/attestations", &response); err != nil {
		return 0, err
	}
	return len(response.Data), nil
}

func (client *Client) SpecUint(ctx context.Context, name string) (uint64, error) {
	var response struct {
		Data map[string]string `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/config/spec", &response); err != nil {
		return 0, err
	}
	value, ok := response.Data[name]
	if !ok {
		return 0, fmt.Errorf("consensus spec does not define %s", name)
	}
	return decimal(name, value)
}

func (client *Client) Liveness(ctx context.Context, epoch uint64, indices []uint64) ([]ValidatorLiveness, error) {
	request := make([]string, len(indices))
	for index, value := range indices {
		request[index] = strconv.FormatUint(value, 10)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []struct {
			Index  string `json:"index"`
			IsLive bool   `json:"is_live"`
		} `json:"data"`
	}
	path := "/qrl/v1/validator/liveness/" + strconv.FormatUint(epoch, 10)
	if err := client.do(ctx, http.MethodPost, path, bytes.NewReader(payload), &response); err != nil {
		return nil, err
	}
	result := make([]ValidatorLiveness, len(response.Data))
	for index, item := range response.Data {
		validatorIndex, err := decimal("validator index", item.Index)
		if err != nil {
			return nil, err
		}
		result[index] = ValidatorLiveness{Index: validatorIndex, IsLive: item.IsLive}
	}
	return result, nil
}

func (client *Client) BlockGraffiti(ctx context.Context, blockID string) (string, error) {
	var response struct {
		Data struct {
			Message struct {
				Body struct {
					Graffiti string `json:"graffiti"`
				} `json:"body"`
			} `json:"message"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/blocks/"+url.PathEscape(blockID), &response); err != nil {
		return "", err
	}
	return response.Data.Message.Body.Graffiti, nil
}

func (client *Client) BlockExecutionPayload(ctx context.Context, blockID string) (ExecutionPayload, error) {
	var response struct {
		Data struct {
			Message struct {
				Body struct {
					ExecutionPayload struct {
						ParentHash   string   `json:"parent_hash"`
						FeeRecipient string   `json:"fee_recipient"`
						BlockNumber  string   `json:"block_number"`
						GasLimit     string   `json:"gas_limit"`
						GasUsed      string   `json:"gas_used"`
						BlockHash    string   `json:"block_hash"`
						Transactions []string `json:"transactions"`
						Withdrawals  []struct {
							Index          string `json:"index"`
							ValidatorIndex string `json:"validator_index"`
							Address        string `json:"address"`
							Amount         string `json:"amount"`
						} `json:"withdrawals"`
					} `json:"execution_payload"`
				} `json:"body"`
			} `json:"message"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/blocks/"+url.PathEscape(blockID), &response); err != nil {
		return ExecutionPayload{}, err
	}
	raw := response.Data.Message.Body.ExecutionPayload
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
	var response struct {
		Data struct {
			Message struct {
				Slot          string `json:"slot"`
				ProposerIndex string `json:"proposer_index"`
				ParentRoot    string `json:"parent_root"`
				StateRoot     string `json:"state_root"`
				Body          struct {
					ExecutionData struct {
						DepositRoot  string `json:"deposit_root"`
						DepositCount string `json:"deposit_count"`
						BlockHash    string `json:"block_hash"`
					} `json:"execution_data"`
					SyncAggregate struct {
						Bits       string   `json:"sync_committee_bits"`
						Signatures []string `json:"sync_committee_signatures"`
					} `json:"sync_aggregate"`
				} `json:"body"`
			} `json:"message"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/blocks/"+url.PathEscape(blockID), &response); err != nil {
		return BlockConsensusData{}, err
	}
	message := response.Data.Message
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
	var response struct {
		Data struct {
			Message struct {
				Body struct {
					Deposits []struct {
						Data struct {
							PublicKey             string `json:"pubkey"`
							WithdrawalCredentials string `json:"withdrawal_credentials"`
							Amount                string `json:"amount"`
							Signature             string `json:"signature"`
						} `json:"data"`
					} `json:"deposits"`
					VoluntaryExits []struct {
						Message struct {
							ValidatorIndex string `json:"validator_index"`
						} `json:"message"`
					} `json:"voluntary_exits"`
					ProposerSlashings []struct {
						Header struct {
							Message struct {
								ProposerIndex string `json:"proposer_index"`
							} `json:"message"`
						} `json:"signed_header_1"`
					} `json:"proposer_slashings"`
					AttesterSlashings []struct {
						Attestation struct {
							Indices []string `json:"attesting_indices"`
						} `json:"attestation_1"`
					} `json:"attester_slashings"`
					ExecutionPayload struct {
						Withdrawals []struct {
							Index          string `json:"index"`
							ValidatorIndex string `json:"validator_index"`
							Address        string `json:"address"`
							Amount         string `json:"amount"`
						} `json:"withdrawals"`
					} `json:"execution_payload"`
				} `json:"body"`
			} `json:"message"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/blocks/"+url.PathEscape(blockID), &response); err != nil {
		return BlockOperations{}, err
	}
	body := response.Data.Message.Body
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
		value, err := decimal("proposer slashing validator index", item.Header.Message.ProposerIndex)
		if err != nil {
			return BlockOperations{}, err
		}
		result.ProposerSlashings = append(result.ProposerSlashings, value)
	}
	for _, item := range body.AttesterSlashings {
		for _, raw := range item.Attestation.Indices {
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
