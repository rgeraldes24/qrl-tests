package beacon

import (
	"encoding/json"
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

type quotedUint64s []uint64

func (values quotedUint64s) MarshalJSON() ([]byte, error) {
	encoded := make([]string, len(values))
	for index, value := range values {
		encoded[index] = strconv.FormatUint(value, 10)
	}
	return json.Marshal(encoded)
}

func (values *quotedUint64s) UnmarshalJSON(input []byte) error {
	var encoded []string
	if err := json.Unmarshal(input, &encoded); err != nil {
		return err
	}
	parsed, err := decimalSlice("quoted integer", encoded)
	if err != nil {
		return err
	}
	*values = parsed
	return nil
}

func (value IndexedAttestation) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		AttestingIndices quotedUint64s   `json:"attesting_indices"`
		Data             AttestationData `json:"data"`
		Signatures       []string        `json:"signatures"`
	}{
		AttestingIndices: value.AttestingIndices,
		Data:             value.Data,
		Signatures:       value.Signatures,
	})
}

func (value *IndexedAttestation) UnmarshalJSON(input []byte) error {
	var decoded struct {
		AttestingIndices quotedUint64s   `json:"attesting_indices"`
		Data             AttestationData `json:"data"`
		Signatures       []string        `json:"signatures"`
	}
	if err := json.Unmarshal(input, &decoded); err != nil {
		return err
	}
	*value = IndexedAttestation{
		AttestingIndices: decoded.AttestingIndices,
		Data:             decoded.Data,
		Signatures:       decoded.Signatures,
	}
	return nil
}

func (value *ValidatorAssignment) UnmarshalJSON(input []byte) error {
	var decoded struct {
		BeaconCommittee quotedUint64s `json:"beaconCommittees"`
		CommitteeIndex  uint64        `json:"committeeIndex,string"`
		AttesterSlot    uint64        `json:"attesterSlot,string"`
		ProposerSlots   quotedUint64s `json:"proposerSlots"`
		ValidatorIndex  uint64        `json:"validatorIndex,string"`
	}
	if err := json.Unmarshal(input, &decoded); err != nil {
		return err
	}
	*value = ValidatorAssignment{
		ValidatorIndex:  decoded.ValidatorIndex,
		CommitteeIndex:  decoded.CommitteeIndex,
		AttesterSlot:    decoded.AttesterSlot,
		ProposerSlots:   decoded.ProposerSlots,
		BeaconCommittee: decoded.BeaconCommittee,
	}
	return nil
}
