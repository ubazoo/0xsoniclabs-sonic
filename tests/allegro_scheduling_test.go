package tests

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/0xsoniclabs/sonic/opera"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestAllegroScheduling_CanProcessTransactionsWithAllegroRules(t *testing.T) {
	const NumRounds = 100
	const NumTxsPerRound = 10

	require := require.New(t)
	net := StartIntegrationTestNet(t, IntegrationTestNetOptions{
		FeatureSet: opera.AllegroFeatures,
		NumNodes:   3, // TODO: build a test with 1 and 3 nodes
	})

	client, err := net.GetClient()
	require.NoError(err)
	chainId, err := client.ChainID(context.Background())
	require.NoError(err)
	defer client.Close()

	signer := types.NewPragueSigner(chainId)

	sponsor := NewAccount()
	_, err = net.EndowAccount(
		sponsor.Address(),
		new(big.Int).Mul(big.NewInt(NumTxsPerRound+1), big.NewInt(1e18)),
	)
	require.NoError(err)

	type account struct {
		account     *Account
		endowmentTx common.Hash
	}

	sponsorNonce := uint64(0)
	accounts := []account{}
	for range NumTxsPerRound {
		account := account{
			account: NewAccount(),
		}

		address := account.account.Address()
		transaction := types.MustSignNewTx(
			sponsor.PrivateKey,
			signer,
			&types.DynamicFeeTx{
				ChainID:   chainId,
				Nonce:     sponsorNonce,
				To:        &address,
				Value:     big.NewInt(1e18),
				Gas:       21000,
				GasFeeCap: big.NewInt(1e11),
			},
		)
		sponsorNonce++
		account.endowmentTx = transaction.Hash()

		require.NoError(client.SendTransaction(context.Background(), transaction))
		accounts = append(accounts, account)
	}

	for i := range accounts {
		receipt, err := net.GetReceipt(accounts[i].endowmentTx)
		require.NoError(err)
		require.Equal(types.ReceiptStatusSuccessful, receipt.Status)
	}

	target := common.Address{0x42}
	for round := range uint64(NumRounds) {
		fmt.Printf("############################### Starting round %d\n", round)
		transactionHashes := []common.Hash{}
		for sender := range NumTxsPerRound {
			transaction := types.MustSignNewTx(
				accounts[sender].account.PrivateKey,
				signer,
				&types.DynamicFeeTx{
					ChainID:   chainId,
					Nonce:     round,
					To:        &target,
					Value:     big.NewInt(1),
					Gas:       21000,
					GasFeeCap: big.NewInt(1e11),
					GasTipCap: big.NewInt(int64(sender) + 1),
				},
			)
			transactionHashes = append(transactionHashes, transaction.Hash())
			require.NoError(client.SendTransaction(context.Background(), transaction))
		}

		for _, hash := range transactionHashes {
			receipt, err := net.GetReceipt(hash)
			require.NoError(err)
			require.Equal(types.ReceiptStatusSuccessful, receipt.Status)
		}
	}

	net.Stop()
}
