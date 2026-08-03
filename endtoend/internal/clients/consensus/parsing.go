package consensus

import (
	"fmt"
	"strconv"
)

func decimal(name, value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return parsed, nil
}

func decimalSlice(name string, values []string) ([]uint64, error) {
	result := make([]uint64, len(values))
	for index, value := range values {
		parsed, err := decimal(name, value)
		if err != nil {
			return nil, err
		}
		result[index] = parsed
	}
	return result, nil
}

func parseValidator(
	indexValue,
	balanceValue,
	status,
	publicKey,
	withdrawal,
	effectiveBalanceValue string,
	slashed bool,
	activationEpochValue,
	exitEpochValue,
	withdrawableEpochValue string,
) (Validator, error) {
	index, err := decimal("validator index", indexValue)
	if err != nil {
		return Validator{}, err
	}
	balance, err := decimal("validator balance", balanceValue)
	if err != nil {
		return Validator{}, err
	}
	effectiveBalance, err := decimal("validator effective balance", effectiveBalanceValue)
	if err != nil {
		return Validator{}, err
	}
	activationEpoch, err := decimal("validator activation epoch", activationEpochValue)
	if err != nil {
		return Validator{}, err
	}
	exitEpoch, err := decimal("validator exit epoch", exitEpochValue)
	if err != nil {
		return Validator{}, err
	}
	withdrawableEpoch, err := decimal("validator withdrawable epoch", withdrawableEpochValue)
	if err != nil {
		return Validator{}, err
	}
	return Validator{
		Index: index, Balance: balance, Status: status,
		PublicKey: publicKey, Withdrawal: withdrawal,
		EffectiveBalance: effectiveBalance, Slashed: slashed,
		ActivationEpoch: activationEpoch, ExitEpoch: exitEpoch, WithdrawableEpoch: withdrawableEpoch,
	}, nil
}
