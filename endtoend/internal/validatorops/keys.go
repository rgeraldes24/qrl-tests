// Package validatorops constructs and submits QRL consensus validator operations.
package validatorops

import (
	"fmt"
	"strconv"

	qrlmisc "github.com/theQRL/go-qrllib/wallet/misc"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"golang.org/x/crypto/sha3"
)

const genesisMnemonic = "veto waiter rail aroma aunt chess fiend than sahara unwary punk dawn belong agent sane reefy loyal from judas clean paste rho madam poor pay convoy duty circa hybrid circus exempt splash"

func GenesisKey(index uint64) (ml_dsa_87.MLDSA87Key, error) {
	seed, err := qrlmisc.MnemonicToBin(genesisMnemonic)
	if err != nil {
		return nil, fmt.Errorf("decode genesis validator mnemonic: %w", err)
	}
	path := "m/12381/238/" + strconv.FormatUint(index, 10) + "/0"
	hash := sha3.NewShake256()
	_, _ = hash.Write(seed)
	_, _ = hash.Write([]byte(path))
	derived := make([]byte, 48)
	if _, err := hash.Read(derived); err != nil {
		return nil, fmt.Errorf("derive genesis validator %d: %w", index, err)
	}
	return ml_dsa_87.SecretKeyFromSeed(derived)
}

func DeterministicKey(marker byte) (ml_dsa_87.MLDSA87Key, error) {
	seed := make([]byte, 48)
	for index := range seed {
		seed[index] = marker + byte(index)
	}
	return ml_dsa_87.SecretKeyFromSeed(seed)
}
