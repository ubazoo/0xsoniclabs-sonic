package blockquery

import (
	"fmt"
	"math"
	"testing"

	"github.com/0xsoniclabs/sonic/scc/light_client/provider"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestBlockQuery_NewBlockQuery_CanInitializeFromUrl(t *testing.T) {
	require := require.New(t)
	blockQuery, err := NewBlockQuery("http://localhost:8545")
	t.Cleanup(blockQuery.Close)
	require.NoError(err)
	require.NotNil(blockQuery)
	require.NotNil(blockQuery.client)
}

func TestServer_NewServer_ReportsErrorForInvalidURL(t *testing.T) {
	require := require.New(t)
	blockQuery, err := NewBlockQuery("not-a-url")
	require.Error(err)
	require.Nil(blockQuery)
}

func TestBlockQuery_Closed_ClosesClient(t *testing.T) {
	require := require.New(t)
	blockQuery, err := NewBlockQuery("http://localhost:8545")
	require.NoError(err)
	require.NotNil(blockQuery.client)
	blockQuery.Close()
	require.Nil(blockQuery.client)
	blockQuery.Close()
	require.Nil(blockQuery.client)
}

func TestBlockQuery_GetBlockInfo_PropagatesClientError(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := provider.NewMockRpcClient(ctrl)
	someError := fmt.Errorf("some error")
	client.EXPECT().Call(
		gomock.Any(), // any result variable
		"eth_getProof",
		"0x1",
		gomock.Any(), // any storage key
		"latest").
		Return(someError)

	blockQuery, err := NewBlockQuery("http://localhost:8545")
	require.NoError(err)
	blockQuery.client = client
	_, err = blockQuery.GetBlockInfo("0x1", math.MaxUint64)
	require.ErrorIs(err, someError)
}

func TestBlockQuery_GetBlockInfo_ReturnsProofQuery(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := provider.NewMockRpcClient(ctrl)
	want := ProofQuery{
		stateRoot: common.Hash{0x42},
		balance:   1,
		nonce:     2,
	}

	client.EXPECT().Call(
		gomock.Any(),
		"eth_getProof",
		"0x1",
		gomock.Any(), // any storage key
		"latest").
		DoAndReturn(
			func(result *ProofQuery, method string, args ...interface{}) error {
				*result = want
				return nil
			})

	blockQuery, err := NewBlockQuery("http://localhost:8545")
	require.NoError(err)
	blockQuery.client = client
	got, err := blockQuery.GetBlockInfo("0x1", math.MaxUint64)
	require.NoError(err)
	require.NotNil(got)
	require.Equal(want, got)
}
