// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type validatorRecordWire struct {
	Index     string        `json:"index"`
	Balance   string        `json:"balance"`
	Status    string        `json:"status"`
	Validator validatorWire `json:"validator"`
}

type validatorWire struct {
	PublicKey         string `json:"pubkey"`
	Withdrawal        string `json:"withdrawal_credentials"`
	EffectiveBalance  string `json:"effective_balance"`
	Slashed           bool   `json:"slashed"`
	ActivationEpoch   string `json:"activation_epoch"`
	ExitEpoch         string `json:"exit_epoch"`
	WithdrawableEpoch string `json:"withdrawable_epoch"`
}

func (item validatorRecordWire) parse() (Validator, error) {
	return parseValidator(
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
}

type validatorIndexWire struct {
	Index string `json:"index"`
}

type depositContractWire struct {
	ChainID string `json:"chain_id"`
	Address string `json:"address"`
}

type validatorLivenessWire struct {
	Index  string `json:"index"`
	IsLive bool   `json:"is_live"`
}

func (client *Client) Validator(ctx context.Context, validatorID string) (Validator, error) {
	var response dataResponse[validatorRecordWire]
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/validators/"+url.PathEscape(validatorID), &response); err != nil {
		return Validator{}, err
	}
	return response.Data.parse()
}

func (client *Client) DepositContract(ctx context.Context) (DepositContract, error) {
	var response dataResponse[depositContractWire]
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
	var response dataResponse[[]validatorIndexWire]
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
	var response dataResponse[[]validatorRecordWire]
	if err := client.get(ctx, path, &response); err != nil {
		return nil, err
	}
	validators := make([]Validator, len(response.Data))
	for index, item := range response.Data {
		validator, err := item.parse()
		if err != nil {
			return nil, err
		}
		validators[index] = validator
	}
	return validators, nil
}

func (client *Client) BlockAttestationCount(ctx context.Context, blockID string) (int, error) {
	var response dataResponse[[]json.RawMessage]
	if err := client.get(ctx, "/qrl/v1/beacon/blocks/"+url.PathEscape(blockID)+"/attestations", &response); err != nil {
		return 0, err
	}
	return len(response.Data), nil
}

func (client *Client) SpecUint(ctx context.Context, name string) (uint64, error) {
	var response dataResponse[map[string]string]
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
	var response dataResponse[[]validatorLivenessWire]
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
