// Package validatorops constructs and submits QRL consensus validator operations.
package validatorops

import (
	"fmt"
	"strconv"

	walletcommon "github.com/theQRL/go-qrllib/wallet/common"
	qrlmisc "github.com/theQRL/go-qrllib/wallet/misc"
	walletmldsa "github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
	"golang.org/x/crypto/sha3"
)

const genesisMnemonic = "veto waiter rail aroma aunt chess fiend than sahara unwary punk dawn belong agent sane reefy loyal from judas clean paste rho madam poor pay convoy duty circa hybrid circus exempt splash"

type Key struct {
	wallet *walletmldsa.Wallet
}

func GenesisKey(index uint64) (*Key, error) {
	seed, err := qrlmisc.MnemonicToBin(genesisMnemonic)
	if err != nil {
		return nil, fmt.Errorf("decode genesis validator mnemonic: %w", err)
	}
	path := "m/12381/238/" + strconv.FormatUint(index, 10) + "/0"
	hash := sha3.NewShake256()
	_, _ = hash.Write(seed)
	_, _ = hash.Write([]byte(path))
	derived := make([]byte, walletcommon.SeedSize)
	if _, err := hash.Read(derived); err != nil {
		return nil, fmt.Errorf("derive genesis validator %d: %w", index, err)
	}
	return keyFromSeed(derived)
}

func DeterministicKey(marker byte) (*Key, error) {
	seed := make([]byte, walletcommon.SeedSize)
	for index := range seed {
		seed[index] = marker + byte(index)
	}
	return keyFromSeed(seed)
}

func keyFromSeed(input []byte) (*Key, error) {
	seed, err := walletcommon.ToSeed(input)
	if err != nil {
		return nil, err
	}
	wallet, err := walletmldsa.NewWalletFromSeed(seed)
	if err != nil {
		return nil, err
	}
	return &Key{wallet: wallet}, nil
}

func (key *Key) PublicKey() []byte {
	publicKey := key.wallet.GetPK()
	return publicKey[:]
}

func (key *Key) Sign(message []byte) ([]byte, error) {
	signature, err := key.wallet.Sign(message)
	if err != nil {
		return nil, err
	}
	return signature[:], nil
}
