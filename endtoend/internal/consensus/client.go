// Package consensus provides the beacon REST operations used by live suites.
package consensus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	VoluntaryExits    []uint64
	ProposerSlashings []uint64
	AttesterSlashings []uint64
	Withdrawals       []Withdrawal
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
	var response struct {
		Data struct {
			Finalized struct {
				Epoch string `json:"epoch"`
			} `json:"finalized"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/finality_checkpoints", &response); err != nil {
		return 0, err
	}
	return decimal("finalized epoch", response.Data.Finalized.Epoch)
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

func (client *Client) BlockOperations(ctx context.Context, blockID string) (BlockOperations, error) {
	var response struct {
		Data struct {
			Message struct {
				Body struct {
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

func (client *Client) Post(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return client.do(ctx, http.MethodPost, path, bytes.NewReader(body), nil)
}

func (client *Client) get(ctx context.Context, path string, result any) error {
	return client.do(ctx, http.MethodGet, path, nil, result)
}

func (client *Client) do(ctx context.Context, method, path string, body io.Reader, result any) error {
	reference, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("parse consensus path %q: %w", path, err)
	}
	endpoint := client.baseURL.ResolveReference(reference)
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return &responseError{
			method: method, path: path, status: response.Status,
			statusCode: response.StatusCode, body: strings.TrimSpace(string(body)),
		}
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(result); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}

func decimal(name, value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return parsed, nil
}
