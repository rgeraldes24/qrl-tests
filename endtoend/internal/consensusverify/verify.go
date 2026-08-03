package consensusverify

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	ssz "github.com/prysmaticlabs/fastssz"
	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	p2ptypes "github.com/theQRL/qrysm/beacon-chain/p2p/types"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/contracts/deposit"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// SignatureSummary records every consensus signature verified in a block.
type SignatureSummary struct {
	Block             int
	Randao            int
	Attestations      int
	SyncCommittee     int
	Deposits          int
	VoluntaryExits    int
	ProposerSlashings int
	AttesterSlashings int
}

func (summary SignatureSummary) Total() int {
	return summary.Block + summary.Randao + summary.Attestations + summary.SyncCommittee +
		summary.Deposits + summary.VoluntaryExits + summary.ProposerSlashings + summary.AttesterSlashings
}

type Verifier struct {
	client  consensusAPI
	genesis consensus.Genesis
	fork    consensus.Fork
	slots   uint64
	pubkeys map[uint64][]byte
}

type consensusAPI interface {
	Block(context.Context, string) (consensus.SignedBlock, error)
	BlockHeader(context.Context, string) (consensus.BlockHeader, error)
	Committee(context.Context, string, uint64, uint64) ([]uint64, error)
	Genesis(context.Context) (consensus.Genesis, error)
	Fork(context.Context) (consensus.Fork, error)
	SpecUint(context.Context, string) (uint64, error)
	SyncCommittee(context.Context, string) ([]uint64, error)
	Validator(context.Context, string) (consensus.Validator, error)
}

func New(ctx context.Context, client consensusAPI) (*Verifier, error) {
	genesis, err := client.Genesis(ctx)
	if err != nil {
		return nil, err
	}
	fork, err := client.Fork(ctx)
	if err != nil {
		return nil, err
	}
	slots, err := client.SpecUint(ctx, "SLOTS_PER_EPOCH")
	if err != nil {
		return nil, err
	}
	return &Verifier{client: client, genesis: genesis, fork: fork, slots: slots, pubkeys: make(map[uint64][]byte)}, nil
}

// VerifyBlock fetches blockID and verifies every consensus signature it carries.
func (verification *Verifier) VerifyBlock(ctx context.Context, blockID string) (SignatureSummary, error) {
	header, err := verification.client.BlockHeader(ctx, blockID)
	if err != nil {
		return SignatureSummary{}, err
	}
	block, err := verification.client.Block(ctx, blockID)
	if err != nil {
		return SignatureSummary{}, err
	}
	return verification.Verify(ctx, header, block)
}

// Verify verifies every consensus signature in an already fetched block.
func (verification *Verifier) Verify(
	ctx context.Context,
	header consensus.BlockHeader,
	block consensus.SignedBlock,
) (SignatureSummary, error) {
	summary := SignatureSummary{}
	if err := verification.verifyBlockHeader(ctx, header.Header.Message, header.Header.Signature, header.Root); err != nil {
		return summary, fmt.Errorf("verify block header signature: %w", err)
	}
	summary.Block++

	slot, err := decimal("block slot", block.Message.Slot)
	if err != nil {
		return summary, err
	}
	proposer, err := decimal("block proposer index", block.Message.ProposerIndex)
	if err != nil {
		return summary, err
	}
	if slotValue, err := decimal("header slot", header.Header.Message.Slot); err != nil || slotValue != slot {
		return summary, fmt.Errorf("block and header slot mismatch")
	}
	if proposerValue, err := decimal("header proposer index", header.Header.Message.ProposerIndex); err != nil || proposerValue != proposer {
		return summary, fmt.Errorf("block and header proposer mismatch")
	}
	if !strings.EqualFold(header.Header.Message.ParentRoot, block.Message.ParentRoot) {
		return summary, fmt.Errorf("block and header parent root mismatch")
	}
	if !strings.EqualFold(header.Header.Message.StateRoot, block.Message.StateRoot) {
		return summary, fmt.Errorf("block and header state root mismatch")
	}
	if !strings.EqualFold(header.Header.Signature, block.Signature) {
		return summary, fmt.Errorf("block and header signature mismatch")
	}
	if err := verification.verifyRandao(ctx, slot, proposer, block.Message.Body.RandaoReveal); err != nil {
		return summary, fmt.Errorf("verify RANDAO signature: %w", err)
	}
	summary.Randao++

	stateID := block.Message.Slot
	for index, attestation := range block.Message.Body.Attestations {
		count, err := verification.verifyAttestation(ctx, attestation)
		if err != nil {
			return summary, fmt.Errorf("verify attestation %d: %w", index, err)
		}
		summary.Attestations += count
	}
	count, err := verification.verifySyncAggregate(
		ctx,
		stateID,
		slot,
		block.Message.ParentRoot,
		block.Message.Body.SyncAggregate.Bits,
		block.Message.Body.SyncAggregate.Signatures,
	)
	if err != nil {
		return summary, fmt.Errorf("verify sync aggregate: %w", err)
	}
	summary.SyncCommittee += count

	depositDomain, err := verification.depositDomain()
	if err != nil {
		return summary, err
	}
	for index, item := range block.Message.Body.Deposits {
		data, err := depositData(item.Data)
		if err != nil {
			return summary, fmt.Errorf("decode deposit %d: %w", index, err)
		}
		if err := deposit.VerifyDepositSignature(data, depositDomain); err != nil {
			return summary, fmt.Errorf("verify deposit %d signature: %w", index, err)
		}
		summary.Deposits++
	}
	for index, item := range block.Message.Body.VoluntaryExits {
		if err := verification.verifyVoluntaryExit(ctx, item.Message, item.Signature); err != nil {
			return summary, fmt.Errorf("verify voluntary exit %d: %w", index, err)
		}
		summary.VoluntaryExits++
	}
	for index, item := range block.Message.Body.ProposerSlashings {
		if err := verification.verifySignedHeader(ctx, item.Header1); err != nil {
			return summary, fmt.Errorf("verify proposer slashing %d header 1: %w", index, err)
		}
		if err := verification.verifySignedHeader(ctx, item.Header2); err != nil {
			return summary, fmt.Errorf("verify proposer slashing %d header 2: %w", index, err)
		}
		summary.ProposerSlashings += 2
	}
	for index, item := range block.Message.Body.AttesterSlashings {
		count1, err := verification.verifyIndexedAttestation(ctx, item.Attestation1)
		if err != nil {
			return summary, fmt.Errorf("verify attester slashing %d attestation 1: %w", index, err)
		}
		count2, err := verification.verifyIndexedAttestation(ctx, item.Attestation2)
		if err != nil {
			return summary, fmt.Errorf("verify attester slashing %d attestation 2: %w", index, err)
		}
		summary.AttesterSlashings += count1 + count2
	}
	return summary, nil
}

func (verification *Verifier) verifyBlockHeader(
	ctx context.Context,
	header consensus.BeaconBlockHeader,
	signatureHex,
	rootHex string,
) error {
	message, proposer, err := beaconBlockHeader(header)
	if err != nil {
		return err
	}
	root, err := message.HashTreeRoot()
	if err != nil {
		return err
	}
	wantRoot, err := decodeFixed("block root", rootHex, fieldparams.RootLength)
	if err != nil {
		return err
	}
	if !bytes.Equal(root[:], wantRoot) {
		return fmt.Errorf("header root mismatch")
	}
	return verification.verifyObject(ctx, message, proposer, uint64(message.Slot)/verification.slots, params.BeaconConfig().DomainBeaconProposer, signatureHex)
}

func (verification *Verifier) verifySignedHeader(ctx context.Context, header consensus.SignedBeaconBlockHeader) error {
	message, proposer, err := beaconBlockHeader(header.Message)
	if err != nil {
		return err
	}
	return verification.verifyObject(ctx, message, proposer, uint64(message.Slot)/verification.slots, params.BeaconConfig().DomainBeaconProposer, header.Signature)
}

func (verification *Verifier) verifyRandao(ctx context.Context, slot, proposer uint64, signatureHex string) error {
	epoch := slot / verification.slots
	value := make([]byte, 32)
	binary.LittleEndian.PutUint64(value, epoch)
	object := p2ptypes.SSZBytes(value)
	return verification.verifyObject(ctx, &object, proposer, epoch, params.BeaconConfig().DomainRandao, signatureHex)
}

func (verification *Verifier) verifyVoluntaryExit(
	ctx context.Context,
	exit consensus.VoluntaryExit,
	signatureHex string,
) error {
	epoch, err := decimal("voluntary exit epoch", exit.Epoch)
	if err != nil {
		return err
	}
	validatorIndex, err := decimal("voluntary exit validator index", exit.ValidatorIndex)
	if err != nil {
		return err
	}
	message := &qrysmpb.VoluntaryExit{Epoch: primitives.Epoch(epoch), ValidatorIndex: primitives.ValidatorIndex(validatorIndex)}
	return verification.verifyObject(ctx, message, validatorIndex, epoch, params.BeaconConfig().DomainVoluntaryExit, signatureHex)
}

func (verification *Verifier) verifyAttestation(
	ctx context.Context,
	attestation consensus.Attestation,
) (int, error) {
	bits, err := decodeHex("attestation aggregation bits", attestation.AggregationBits)
	if err != nil {
		return 0, err
	}
	positions := bitfield.Bitlist(bits).BitIndices()
	committee, err := committee(ctx, verification.client, attestation.Data.Slot, attestation.Data)
	if err != nil {
		return 0, err
	}
	indices := make([]uint64, len(positions))
	for index, position := range positions {
		if int(position) >= len(committee) {
			return 0, fmt.Errorf("aggregation bit %d exceeds committee length %d", position, len(committee))
		}
		indices[index] = committee[position]
	}
	return verification.verifyAttestationSignatures(ctx, attestation.Data, indices, attestation.Signatures)
}

func (verification *Verifier) verifyIndexedAttestation(
	ctx context.Context,
	attestation consensus.IndexedAttestation,
) (int, error) {
	indices := make([]uint64, len(attestation.AttestingIndices))
	for index, value := range attestation.AttestingIndices {
		parsed, err := decimal("attesting index", value)
		if err != nil {
			return 0, err
		}
		indices[index] = parsed
	}
	return verification.verifyAttestationSignatures(ctx, attestation.Data, indices, attestation.Signatures)
}

func (verification *Verifier) verifyAttestationSignatures(
	ctx context.Context,
	data consensus.AttestationData,
	indices []uint64,
	signatures []string,
) (int, error) {
	if len(indices) != len(signatures) {
		return 0, fmt.Errorf("attestation has %d participants and %d signatures", len(indices), len(signatures))
	}
	message, targetEpoch, err := attestationData(data)
	if err != nil {
		return 0, err
	}
	for index, validatorIndex := range indices {
		if err := verification.verifyObject(
			ctx,
			message,
			validatorIndex,
			targetEpoch,
			params.BeaconConfig().DomainBeaconAttester,
			signatures[index],
		); err != nil {
			return index, fmt.Errorf("participant %d: %w", validatorIndex, err)
		}
	}
	return len(signatures), nil
}

func (verification *Verifier) verifySyncAggregate(
	ctx context.Context,
	stateID string,
	slot uint64,
	parentRootHex,
	bitsHex string,
	signatures []string,
) (int, error) {
	bits, err := decodeHex("sync committee bits", bitsHex)
	if err != nil {
		return 0, err
	}
	committee, err := verification.client.SyncCommittee(ctx, stateID)
	if err != nil {
		return 0, err
	}
	participants := make([]uint64, 0, len(committee))
	for index, validatorIndex := range committee {
		if index/8 < len(bits) && bits[index/8]&(1<<uint(index%8)) != 0 {
			participants = append(participants, validatorIndex)
		}
	}
	if len(participants) != len(signatures) {
		return 0, fmt.Errorf("sync aggregate has %d participants and %d signatures", len(participants), len(signatures))
	}
	parentRoot, err := decodeFixed("parent block root", parentRootHex, fieldparams.RootLength)
	if err != nil {
		return 0, err
	}
	object := p2ptypes.SSZBytes(parentRoot)
	epoch := uint64(0)
	if slot > 0 {
		epoch = (slot - 1) / verification.slots
	}
	for index, validatorIndex := range participants {
		if err := verification.verifyObject(
			ctx,
			&object,
			validatorIndex,
			epoch,
			params.BeaconConfig().DomainSyncCommittee,
			signatures[index],
		); err != nil {
			return index, fmt.Errorf("participant %d: %w", validatorIndex, err)
		}
	}
	return len(signatures), nil
}

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
	signature, err := decodeFixed("signature", signatureHex, fieldparams.MLDSA87SignatureLength)
	if err != nil {
		return err
	}
	domain, err := verification.domain(domainType, epoch)
	if err != nil {
		return err
	}
	return signing.VerifySigningRoot(object, publicKey, signature, domain)
}

func (verification *Verifier) publicKey(ctx context.Context, validatorIndex uint64) ([]byte, error) {
	if publicKey := verification.pubkeys[validatorIndex]; publicKey != nil {
		return publicKey, nil
	}
	validator, err := verification.client.Validator(ctx, strconv.FormatUint(validatorIndex, 10))
	if err != nil {
		return nil, err
	}
	publicKey, err := decodeFixed("validator public key", validator.PublicKey, fieldparams.MLDSA87PubkeyLength)
	if err != nil {
		return nil, err
	}
	verification.pubkeys[validatorIndex] = publicKey
	return publicKey, nil
}

func (verification *Verifier) domain(domainType [4]byte, epoch uint64) ([]byte, error) {
	version := verification.fork.CurrentVersion
	if epoch < verification.fork.Epoch {
		version = verification.fork.PreviousVersion
	}
	forkVersion, err := decodeFixed("fork version", version, fieldparams.VersionLength)
	if err != nil {
		return nil, err
	}
	genesisRoot, err := decodeFixed("genesis validators root", verification.genesis.ValidatorsRoot, fieldparams.RootLength)
	if err != nil {
		return nil, err
	}
	return signing.ComputeDomain(domainType, forkVersion, genesisRoot)
}

func (verification *Verifier) depositDomain() ([]byte, error) {
	forkVersion, err := decodeFixed("genesis fork version", verification.genesis.ForkVersion, fieldparams.VersionLength)
	if err != nil {
		return nil, err
	}
	return signing.ComputeDomain(params.BeaconConfig().DomainDeposit, forkVersion, nil)
}
