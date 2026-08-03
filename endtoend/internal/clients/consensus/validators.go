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
	Index     uint64        `json:"index,string"`
	Balance   uint64        `json:"balance,string"`
	Status    string        `json:"status"`
	Validator validatorWire `json:"validator"`
}

type validatorWire struct {
	PublicKey         string `json:"pubkey"`
	Withdrawal        string `json:"withdrawal_credentials"`
	EffectiveBalance  uint64 `json:"effective_balance,string"`
	Slashed           bool   `json:"slashed"`
	ActivationEpoch   uint64 `json:"activation_epoch,string"`
	ExitEpoch         uint64 `json:"exit_epoch,string"`
	WithdrawableEpoch uint64 `json:"withdrawable_epoch,string"`
}

func (item validatorRecordWire) validator() Validator {
	return Validator{
		Index: item.Index, Balance: item.Balance, Status: item.Status,
		PublicKey: item.Validator.PublicKey, Withdrawal: item.Validator.Withdrawal,
		EffectiveBalance: item.Validator.EffectiveBalance, Slashed: item.Validator.Slashed,
		ActivationEpoch: item.Validator.ActivationEpoch, ExitEpoch: item.Validator.ExitEpoch,
		WithdrawableEpoch: item.Validator.WithdrawableEpoch,
	}
}

type validatorIndexWire struct {
	Index uint64 `json:"index,string"`
}

func (client *Client) Validator(ctx context.Context, validatorID string) (Validator, error) {
	var response dataResponse[validatorRecordWire]
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/validators/"+url.PathEscape(validatorID), &response); err != nil {
		return Validator{}, err
	}
	return response.Data.validator(), nil
}

func (client *Client) DepositContract(ctx context.Context) (DepositContract, error) {
	var response dataResponse[DepositContract]
	if err := client.get(ctx, "/qrl/v1/config/deposit_contract", &response); err != nil {
		return DepositContract{}, err
	}
	return response.Data, nil
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
		indices[index] = validator.Index
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
		validators[index] = item.validator()
	}
	return validators, nil
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
	var response dataResponse[[]ValidatorLiveness]
	path := "/qrl/v1/validator/liveness/" + strconv.FormatUint(epoch, 10)
	if err := client.do(ctx, http.MethodPost, path, bytes.NewReader(payload), &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}
