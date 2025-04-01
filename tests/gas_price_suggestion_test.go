package tests

import (
	"math/big"
	"testing"

	"github.com/0xsoniclabs/sonic/opera"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGasPrice_SuggestedGasPricesApproximateActualBaseFees(t *testing.T) {
	require := require.New(t)
	net, client := makeNetAndClient(t)

	fees := []uint64{}
	suggestions := []uint64{}
	for i := 0; i < 10; i++ {
		suggestedPrice, err := client.SuggestGasPrice(t.Context())
		require.NoError(err)

		// new block
		receipt, err := net.EndowAccount(common.Address{42}, big.NewInt(100))
		require.NoError(err)

		lastBlock, err := client.BlockByNumber(t.Context(), receipt.BlockNumber)
		require.NoError(err)

		// store suggested and actual prices.
		suggestions = append(suggestions, suggestedPrice.Uint64())
		fees = append(fees, lastBlock.BaseFee().Uint64())
	}

	// Suggestions should over-estimate the actual prices by ~10%
	for i := 1; i < int(len(suggestions)); i++ {
		ratio := float64(suggestions[i]) / float64(fees[i-1])
		require.Less(1.09, ratio, "step %d, suggestion %d, fees %d", i, suggestions[i], fees[i-1])
		require.Less(ratio, 1.11, "step %d, suggestion %d, fees %d", i, suggestions[i], fees[i-1])
	}
}

func TestGasPrice_UnderpricedTransactionsAreRejected(t *testing.T) {
	net, client := makeNetAndClient(t)

	chainId, err := client.ChainID(t.Context())
	require.NoError(t, err)

	sender := net.net.GetSessionSponsor()

	// Note: Use a second account to generate eip-7702 authorizations. Otherwise
	// transactions from the same sender are limited to one in flight, and the
	// test cannot be completed.
	authorizer := NewAccount()
	auth, err := types.SignSetCode(authorizer.PrivateKey, types.SetCodeAuthorization{
		Address: common.Address{42},
		Nonce:   0,
		ChainID: *uint256.MustFromBig(chainId),
	})
	require.NoError(t, err)

	send := func(data types.TxData) error {
		tx := signTransaction(t, chainId, data, sender)
		return client.SendTransaction(t.Context(), tx)
	}

	lastBlock, err := client.BlockByNumber(t.Context(), nil)
	require.NoError(t, err)

	// Everything below ~5% above the base fee should be rejected.
	baseFee := int(lastBlock.BaseFee().Uint64())
	for _, extra := range []int{-10, 0, baseFee / 100, 4 * baseFee / 100} {
		feeCap := int64(baseFee + extra)

		legacyTx := setTransactionDefaults(t, net, &types.LegacyTx{}, sender)
		legacyTx.GasPrice = big.NewInt(feeCap)
		assert.ErrorContains(t, send(legacyTx), "transaction underpriced", "legacy tx, %d", baseFee)

		accessListTx := setTransactionDefaults(t, net, &types.AccessListTx{}, sender)
		accessListTx.GasPrice = big.NewInt(feeCap)
		assert.ErrorContains(t, send(accessListTx), "transaction underpriced", "access list tx, %d", baseFee)

		dynamicFeeTx := setTransactionDefaults(t, net, &types.DynamicFeeTx{}, sender)
		dynamicFeeTx.GasFeeCap = big.NewInt(feeCap)
		assert.ErrorContains(t, send(dynamicFeeTx), "transaction underpriced", "dynamic fee tx, %d", baseFee)

		blobTx := setTransactionDefaults(t, net, &types.BlobTx{}, sender)
		blobTx.GasFeeCap = uint256.NewInt(uint64(feeCap))
		assert.ErrorContains(t, send(blobTx), "transaction underpriced", "blob tx, %d", baseFee)

		setCodeTx := setTransactionDefaults(t, net, &types.SetCodeTx{
			AuthList: []types.SetCodeAuthorization{auth},
		}, sender)
		setCodeTx.GasFeeCap = uint256.NewInt(uint64(feeCap))
		assert.ErrorContains(t, send(setCodeTx), "transaction underpriced", "set code tx, %d", baseFee)
	}

	// Everything over ~5% above the base fee should be accepted.
	feeCap := int64(baseFee + 7*baseFee/100)
	nonce, err := client.NonceAt(t.Context(), sender.Address(), nil)
	require.NoError(t, err)

	legacyTx := setTransactionDefaults(t, net, &types.LegacyTx{Nonce: nonce + 0}, sender)
	legacyTx.GasPrice = big.NewInt(feeCap)
	assert.NoError(t, send(legacyTx), "legacy tx, %d", baseFee)

	accessListTx := setTransactionDefaults(t, net, &types.AccessListTx{Nonce: nonce + 1}, sender)
	accessListTx.GasPrice = big.NewInt(feeCap)
	assert.NoError(t, send(accessListTx), "access list tx, %d", baseFee)

	dynamicFeeTx := setTransactionDefaults(t, net, &types.DynamicFeeTx{Nonce: nonce + 2}, sender)
	dynamicFeeTx.GasFeeCap = big.NewInt(feeCap)
	assert.NoError(t, send(dynamicFeeTx), "dynamic fee tx, %d", baseFee)

	blobTx := setTransactionDefaults(t, net, &types.BlobTx{Nonce: nonce + 3}, sender)
	blobTx.GasFeeCap = uint256.NewInt(uint64(feeCap))
	assert.NoError(t, send(blobTx), "blob tx, %d", baseFee)

	setCodeTx := setTransactionDefaults(t, net, &types.SetCodeTx{
		Nonce:    nonce + 4,
		AuthList: []types.SetCodeAuthorization{auth},
	}, sender)
	setCodeTx.GasFeeCap = uint256.NewInt(uint64(feeCap))
	assert.NoError(t, send(setCodeTx), "set code tx, %d", baseFee)
}

func makeNetAndClient(t *testing.T) (*IntegrationTestNet, *ethclient.Client) {
	net := StartIntegrationTestNet(t,
		IntegrationTestNetOptions{FeatureSet: opera.AllegroFeatures})

	client, err := net.GetClient()
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	return net, client
}
