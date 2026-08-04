// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensus

import (
	"context"
	"fmt"
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
	record, err := getData[validatorRecordWire](ctx, client, "/qrl/v1/beacon/states/head/validators/"+url.PathEscape(validatorID))
	if err != nil {
		return Validator{}, err
	}
	return record.validator(), nil
}

func (client *Client) DepositContract(ctx context.Context) (DepositContract, error) {
	return getData[DepositContract](ctx, client, "/qrl/v1/config/deposit_contract")
}

func (client *Client) ActiveValidatorCount(ctx context.Context) (int, error) {
	indices, err := client.ActiveValidatorIndices(ctx)
	return len(indices), err
}

func (client *Client) ActiveValidatorIndices(ctx context.Context) ([]uint64, error) {
	records, err := getData[[]validatorIndexWire](ctx, client, "/qrl/v1/beacon/states/head/validators?status=active")
	if err != nil {
		return nil, err
	}
	indices := make([]uint64, len(records))
	for index, validator := range records {
		indices[index] = validator.Index
	}
	return indices, nil
}

func (client *Client) Validators(ctx context.Context, status string) ([]Validator, error) {
	path := "/qrl/v1/beacon/states/head/validators"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	records, err := getData[[]validatorRecordWire](ctx, client, path)
	if err != nil {
		return nil, err
	}
	validators := make([]Validator, len(records))
	for index, item := range records {
		validators[index] = item.validator()
	}
	return validators, nil
}

func (client *Client) SpecUint(ctx context.Context, name string) (uint64, error) {
	values, err := getData[map[string]string](ctx, client, "/qrl/v1/config/spec")
	if err != nil {
		return 0, err
	}
	value, ok := values[name]
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
	path := "/qrl/v1/validator/liveness/" + strconv.FormatUint(epoch, 10)
	return postData[[]ValidatorLiveness](ctx, client, path, request)
}
