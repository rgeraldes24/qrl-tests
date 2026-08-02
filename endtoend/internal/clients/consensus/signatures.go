package consensus

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

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

type signatureContext struct {
	genesis Genesis
	fork    Fork
	slots   uint64
	pubkeys map[uint64][]byte
}

type signedHeaderResponse struct {
	Data struct {
		Root      string `json:"root"`
		Canonical bool   `json:"canonical"`
		Header    struct {
			Message   blockHeaderJSON `json:"message"`
			Signature string          `json:"signature"`
		} `json:"header"`
	} `json:"data"`
}

type blockHeaderJSON struct {
	Slot          string `json:"slot"`
	ProposerIndex string `json:"proposer_index"`
	ParentRoot    string `json:"parent_root"`
	StateRoot     string `json:"state_root"`
	BodyRoot      string `json:"body_root"`
}

type signedBlockResponse struct {
	Data struct {
		Message struct {
			Slot          string `json:"slot"`
			ProposerIndex string `json:"proposer_index"`
			ParentRoot    string `json:"parent_root"`
			StateRoot     string `json:"state_root"`
			Body          struct {
				RandaoReveal string            `json:"randao_reveal"`
				Attestations []attestationJSON `json:"attestations"`
				Deposits     []struct {
					Data depositJSON `json:"data"`
				} `json:"deposits"`
				VoluntaryExits []struct {
					Message   voluntaryExitJSON `json:"message"`
					Signature string            `json:"signature"`
				} `json:"voluntary_exits"`
				ProposerSlashings []struct {
					Header1 signedHeaderJSON `json:"signed_header_1"`
					Header2 signedHeaderJSON `json:"signed_header_2"`
				} `json:"proposer_slashings"`
				AttesterSlashings []struct {
					Attestation1 indexedAttestationJSON `json:"attestation_1"`
					Attestation2 indexedAttestationJSON `json:"attestation_2"`
				} `json:"attester_slashings"`
				SyncAggregate struct {
					Bits       string   `json:"sync_committee_bits"`
					Signatures []string `json:"sync_committee_signatures"`
				} `json:"sync_aggregate"`
			} `json:"body"`
		} `json:"message"`
		Signature string `json:"signature"`
	} `json:"data"`
}

type signedHeaderJSON struct {
	Message   blockHeaderJSON `json:"message"`
	Signature string          `json:"signature"`
}

type checkpointJSON struct {
	Epoch string `json:"epoch"`
	Root  string `json:"root"`
}

type attestationDataJSON struct {
	Slot            string         `json:"slot"`
	CommitteeIndex  string         `json:"index"`
	BeaconBlockRoot string         `json:"beacon_block_root"`
	Source          checkpointJSON `json:"source"`
	Target          checkpointJSON `json:"target"`
}

type attestationJSON struct {
	AggregationBits string              `json:"aggregation_bits"`
	Data            attestationDataJSON `json:"data"`
	Signatures      []string            `json:"signatures"`
}

type indexedAttestationJSON struct {
	AttestingIndices []string            `json:"attesting_indices"`
	Data             attestationDataJSON `json:"data"`
	Signatures       []string            `json:"signatures"`
}

type voluntaryExitJSON struct {
	Epoch          string `json:"epoch"`
	ValidatorIndex string `json:"validator_index"`
}

type depositJSON struct {
	PublicKey             string `json:"pubkey"`
	WithdrawalCredentials string `json:"withdrawal_credentials"`
	Amount                string `json:"amount"`
	Signature             string `json:"signature"`
}

// VerifyBlockSignatures verifies every consensus signature carried by blockID.
func (client *Client) VerifyBlockSignatures(ctx context.Context, blockID string) (SignatureSummary, error) {
	verification, err := client.newSignatureContext(ctx)
	if err != nil {
		return SignatureSummary{}, err
	}
	var header signedHeaderResponse
	if err := client.get(ctx, "/qrl/v1/beacon/headers/"+url.PathEscape(blockID), &header); err != nil {
		return SignatureSummary{}, err
	}
	var block signedBlockResponse
	if err := client.get(ctx, "/qrl/v1/beacon/blocks/"+url.PathEscape(blockID), &block); err != nil {
		return SignatureSummary{}, err
	}

	summary := SignatureSummary{}
	if err := verification.verifyBlockHeader(ctx, client, header.Data.Header.Message, header.Data.Header.Signature, header.Data.Root); err != nil {
		return summary, fmt.Errorf("verify block header signature: %w", err)
	}
	summary.Block++

	slot, err := decimal("block slot", block.Data.Message.Slot)
	if err != nil {
		return summary, err
	}
	proposer, err := decimal("block proposer index", block.Data.Message.ProposerIndex)
	if err != nil {
		return summary, err
	}
	if slotValue, err := decimal("header slot", header.Data.Header.Message.Slot); err != nil || slotValue != slot {
		return summary, fmt.Errorf("block and header slot mismatch")
	}
	if proposerValue, err := decimal("header proposer index", header.Data.Header.Message.ProposerIndex); err != nil || proposerValue != proposer {
		return summary, fmt.Errorf("block and header proposer mismatch")
	}
	if !strings.EqualFold(header.Data.Header.Message.ParentRoot, block.Data.Message.ParentRoot) {
		return summary, fmt.Errorf("block and header parent root mismatch")
	}
	if !strings.EqualFold(header.Data.Header.Message.StateRoot, block.Data.Message.StateRoot) {
		return summary, fmt.Errorf("block and header state root mismatch")
	}
	if !strings.EqualFold(header.Data.Header.Signature, block.Data.Signature) {
		return summary, fmt.Errorf("block and header signature mismatch")
	}
	if err := verification.verifyRandao(ctx, client, slot, proposer, block.Data.Message.Body.RandaoReveal); err != nil {
		return summary, fmt.Errorf("verify RANDAO signature: %w", err)
	}
	summary.Randao++

	stateID := block.Data.Message.Slot
	for index, attestation := range block.Data.Message.Body.Attestations {
		count, err := verification.verifyAttestation(ctx, client, attestation)
		if err != nil {
			return summary, fmt.Errorf("verify attestation %d: %w", index, err)
		}
		summary.Attestations += count
	}
	count, err := verification.verifySyncAggregate(
		ctx,
		client,
		stateID,
		slot,
		block.Data.Message.ParentRoot,
		block.Data.Message.Body.SyncAggregate.Bits,
		block.Data.Message.Body.SyncAggregate.Signatures,
	)
	if err != nil {
		return summary, fmt.Errorf("verify sync aggregate: %w", err)
	}
	summary.SyncCommittee += count

	depositDomain, err := verification.depositDomain()
	if err != nil {
		return summary, err
	}
	for index, item := range block.Data.Message.Body.Deposits {
		data, err := depositData(item.Data)
		if err != nil {
			return summary, fmt.Errorf("decode deposit %d: %w", index, err)
		}
		if err := deposit.VerifyDepositSignature(data, depositDomain); err != nil {
			return summary, fmt.Errorf("verify deposit %d signature: %w", index, err)
		}
		summary.Deposits++
	}
	for index, item := range block.Data.Message.Body.VoluntaryExits {
		if err := verification.verifyVoluntaryExit(ctx, client, item.Message, item.Signature); err != nil {
			return summary, fmt.Errorf("verify voluntary exit %d: %w", index, err)
		}
		summary.VoluntaryExits++
	}
	for index, item := range block.Data.Message.Body.ProposerSlashings {
		if err := verification.verifySignedHeader(ctx, client, item.Header1); err != nil {
			return summary, fmt.Errorf("verify proposer slashing %d header 1: %w", index, err)
		}
		if err := verification.verifySignedHeader(ctx, client, item.Header2); err != nil {
			return summary, fmt.Errorf("verify proposer slashing %d header 2: %w", index, err)
		}
		summary.ProposerSlashings += 2
	}
	for index, item := range block.Data.Message.Body.AttesterSlashings {
		count1, err := verification.verifyIndexedAttestation(ctx, client, item.Attestation1)
		if err != nil {
			return summary, fmt.Errorf("verify attester slashing %d attestation 1: %w", index, err)
		}
		count2, err := verification.verifyIndexedAttestation(ctx, client, item.Attestation2)
		if err != nil {
			return summary, fmt.Errorf("verify attester slashing %d attestation 2: %w", index, err)
		}
		summary.AttesterSlashings += count1 + count2
	}
	return summary, nil
}

func (client *Client) newSignatureContext(ctx context.Context) (*signatureContext, error) {
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
	return &signatureContext{genesis: genesis, fork: fork, slots: slots, pubkeys: make(map[uint64][]byte)}, nil
}

func (verification *signatureContext) verifyBlockHeader(
	ctx context.Context,
	client *Client,
	header blockHeaderJSON,
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
	if !equalBytes(root[:], wantRoot) {
		return fmt.Errorf("header root mismatch")
	}
	return verification.verifyObject(ctx, client, message, proposer, uint64(message.Slot)/verification.slots, params.BeaconConfig().DomainBeaconProposer, signatureHex)
}

func (verification *signatureContext) verifySignedHeader(ctx context.Context, client *Client, header signedHeaderJSON) error {
	message, proposer, err := beaconBlockHeader(header.Message)
	if err != nil {
		return err
	}
	return verification.verifyObject(ctx, client, message, proposer, uint64(message.Slot)/verification.slots, params.BeaconConfig().DomainBeaconProposer, header.Signature)
}

func (verification *signatureContext) verifyRandao(ctx context.Context, client *Client, slot, proposer uint64, signatureHex string) error {
	epoch := slot / verification.slots
	value := make([]byte, 32)
	binary.LittleEndian.PutUint64(value, epoch)
	object := p2ptypes.SSZBytes(value)
	return verification.verifyObject(ctx, client, &object, proposer, epoch, params.BeaconConfig().DomainRandao, signatureHex)
}

func (verification *signatureContext) verifyVoluntaryExit(
	ctx context.Context,
	client *Client,
	exit voluntaryExitJSON,
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
	return verification.verifyObject(ctx, client, message, validatorIndex, epoch, params.BeaconConfig().DomainVoluntaryExit, signatureHex)
}

func (verification *signatureContext) verifyAttestation(
	ctx context.Context,
	client *Client,
	attestation attestationJSON,
) (int, error) {
	bits, err := decodeHex("attestation aggregation bits", attestation.AggregationBits)
	if err != nil {
		return 0, err
	}
	positions := bitfield.Bitlist(bits).BitIndices()
	committee, err := client.committee(ctx, attestation.Data.Slot, attestation.Data)
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
	return verification.verifyAttestationSignatures(ctx, client, attestation.Data, indices, attestation.Signatures)
}

func (verification *signatureContext) verifyIndexedAttestation(
	ctx context.Context,
	client *Client,
	attestation indexedAttestationJSON,
) (int, error) {
	indices := make([]uint64, len(attestation.AttestingIndices))
	for index, value := range attestation.AttestingIndices {
		parsed, err := decimal("attesting index", value)
		if err != nil {
			return 0, err
		}
		indices[index] = parsed
	}
	return verification.verifyAttestationSignatures(ctx, client, attestation.Data, indices, attestation.Signatures)
}

func (verification *signatureContext) verifyAttestationSignatures(
	ctx context.Context,
	client *Client,
	data attestationDataJSON,
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
			client,
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

func (verification *signatureContext) verifySyncAggregate(
	ctx context.Context,
	client *Client,
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
	committee, err := client.syncCommittee(ctx, stateID)
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
			client,
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

func (verification *signatureContext) verifyObject(
	ctx context.Context,
	client *Client,
	object ssz.HashRoot,
	validatorIndex,
	epoch uint64,
	domainType [4]byte,
	signatureHex string,
) error {
	publicKey, err := verification.publicKey(ctx, client, validatorIndex)
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

func (verification *signatureContext) publicKey(ctx context.Context, client *Client, validatorIndex uint64) ([]byte, error) {
	if publicKey := verification.pubkeys[validatorIndex]; publicKey != nil {
		return publicKey, nil
	}
	validator, err := client.Validator(ctx, strconv.FormatUint(validatorIndex, 10))
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

func (verification *signatureContext) domain(domainType [4]byte, epoch uint64) ([]byte, error) {
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

func (verification *signatureContext) depositDomain() ([]byte, error) {
	forkVersion, err := decodeFixed("genesis fork version", verification.genesis.ForkVersion, fieldparams.VersionLength)
	if err != nil {
		return nil, err
	}
	return signing.ComputeDomain(params.BeaconConfig().DomainDeposit, forkVersion, nil)
}

func (client *Client) committee(ctx context.Context, stateID string, data attestationDataJSON) ([]uint64, error) {
	slot, err := decimal("attestation slot", data.Slot)
	if err != nil {
		return nil, err
	}
	index, err := decimal("attestation committee index", data.CommitteeIndex)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []struct {
			Index      string   `json:"index"`
			Slot       string   `json:"slot"`
			Validators []string `json:"validators"`
		} `json:"data"`
	}
	path := fmt.Sprintf(
		"/qrl/v1/beacon/states/%s/committees?slot=%d&index=%d",
		url.PathEscape(stateID),
		slot,
		index,
	)
	if err := client.get(ctx, path, &response); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 {
		return nil, fmt.Errorf("expected one committee, got %d", len(response.Data))
	}
	return decimalSlice("committee validator", response.Data[0].Validators)
}

func (client *Client) syncCommittee(ctx context.Context, stateID string) ([]uint64, error) {
	var response struct {
		Data struct {
			Validators []string `json:"validators"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/states/"+url.PathEscape(stateID)+"/sync_committees", &response); err != nil {
		return nil, err
	}
	return decimalSlice("sync committee validator", response.Data.Validators)
}

func beaconBlockHeader(value blockHeaderJSON) (*qrysmpb.BeaconBlockHeader, uint64, error) {
	slot, err := decimal("block header slot", value.Slot)
	if err != nil {
		return nil, 0, err
	}
	proposer, err := decimal("block header proposer index", value.ProposerIndex)
	if err != nil {
		return nil, 0, err
	}
	parentRoot, err := decodeFixed("block header parent root", value.ParentRoot, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	stateRoot, err := decodeFixed("block header state root", value.StateRoot, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	bodyRoot, err := decodeFixed("block header body root", value.BodyRoot, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	return &qrysmpb.BeaconBlockHeader{
		Slot: primitives.Slot(slot), ProposerIndex: primitives.ValidatorIndex(proposer),
		ParentRoot: parentRoot, StateRoot: stateRoot, BodyRoot: bodyRoot,
	}, proposer, nil
}

func attestationData(value attestationDataJSON) (*qrysmpb.AttestationData, uint64, error) {
	slot, err := decimal("attestation slot", value.Slot)
	if err != nil {
		return nil, 0, err
	}
	committeeIndex, err := decimal("attestation committee index", value.CommitteeIndex)
	if err != nil {
		return nil, 0, err
	}
	targetEpoch, err := decimal("attestation target epoch", value.Target.Epoch)
	if err != nil {
		return nil, 0, err
	}
	sourceEpoch, err := decimal("attestation source epoch", value.Source.Epoch)
	if err != nil {
		return nil, 0, err
	}
	beaconRoot, err := decodeFixed("attestation beacon block root", value.BeaconBlockRoot, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	sourceRoot, err := decodeFixed("attestation source root", value.Source.Root, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	targetRoot, err := decodeFixed("attestation target root", value.Target.Root, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	return &qrysmpb.AttestationData{
		Slot: primitives.Slot(slot), CommitteeIndex: primitives.CommitteeIndex(committeeIndex), BeaconBlockRoot: beaconRoot,
		Source: &qrysmpb.Checkpoint{Epoch: primitives.Epoch(sourceEpoch), Root: sourceRoot},
		Target: &qrysmpb.Checkpoint{Epoch: primitives.Epoch(targetEpoch), Root: targetRoot},
	}, targetEpoch, nil
}

func depositData(value depositJSON) (*qrysmpb.Deposit_Data, error) {
	publicKey, err := decodeFixed("deposit public key", value.PublicKey, fieldparams.MLDSA87PubkeyLength)
	if err != nil {
		return nil, err
	}
	withdrawalCredentials, err := decodeFixed("deposit withdrawal credentials", value.WithdrawalCredentials, fieldparams.FeeRecipientLength)
	if err != nil {
		return nil, err
	}
	amount, err := decimal("deposit amount", value.Amount)
	if err != nil {
		return nil, err
	}
	signature, err := decodeFixed("deposit signature", value.Signature, fieldparams.MLDSA87SignatureLength)
	if err != nil {
		return nil, err
	}
	return &qrysmpb.Deposit_Data{
		PublicKey: publicKey, WithdrawalCredentials: withdrawalCredentials, Amount: amount, Signature: signature,
	}, nil
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

func decodeFixed(name, value string, length int) ([]byte, error) {
	decoded, err := decodeHex(name, value)
	if err != nil {
		return nil, err
	}
	if len(decoded) != length {
		return nil, fmt.Errorf("invalid %s length %d, want %d", name, len(decoded), length)
	}
	return decoded, nil
}

func decodeHex(name, value string) ([]byte, error) {
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", name, err)
	}
	return decoded, nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
