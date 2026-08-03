package execfixture

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/vm/runtime"
)

func TestStateInitCode(t *testing.T) {
	topic := FullTopic(0x80)
	runtimeCode, state, err := runtime.Execute(stateInitCode(topic), nil, nil)
	require.NoError(t, err)
	contract := PatternedAddress(0x40)
	state.SetCode(contract, runtimeCode)
	value := FullWord(0x20)
	output, _, err := runtime.Call(contract, value[:], &runtime.Config{State: state})
	require.NoError(t, err)
	require.Equal(t, value, state.GetState(contract, common.Hash{}))
	require.True(t, bytes.Equal(value[:], output))
	require.Len(t, state.Logs(), 1)
	require.Equal(t, topic, state.Logs()[0].Topics[0])
}
