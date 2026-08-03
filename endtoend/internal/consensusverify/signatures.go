// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"context"
	"strconv"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscrypto"
	ssz "github.com/prysmaticlabs/fastssz"
)

func (verification *Verifier) verifyObject(
	ctx context.Context,
	object ssz.HashRoot,
	validatorIndex,
	epoch uint64,
	domainType [4]byte,
	signatureHex string,
) error {
	publicKey, err := verification.publicKey(ctx, validatorIndex)
	if err != nil {
		return err
	}
	signature, err := decodeFixed("signature", signatureHex, consensuscrypto.SignatureLength)
	if err != nil {
		return err
	}
	domain, err := verification.chain.Domain(domainType, epoch)
	if err != nil {
		return err
	}
	return consensuscrypto.VerifySigningRoot(object, publicKey, signature, [consensuscrypto.RootLength]byte(domain))
}

func (verification *Verifier) publicKey(ctx context.Context, validatorIndex uint64) ([]byte, error) {
	if publicKey := verification.pubkeys[validatorIndex]; publicKey != nil {
		return publicKey, nil
	}
	validator, err := verification.client.Validator(ctx, strconv.FormatUint(validatorIndex, 10))
	if err != nil {
		return nil, err
	}
	publicKey, err := decodeFixed("validator public key", validator.PublicKey, consensuscrypto.PublicKeyLength)
	if err != nil {
		return nil, err
	}
	verification.pubkeys[validatorIndex] = publicKey
	return publicKey, nil
}
