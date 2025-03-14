package light_client

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/consensus/inter/idx"
	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/scc/cert"
	bq "github.com/0xsoniclabs/sonic/scc/light_client/block_query"
	"github.com/0xsoniclabs/sonic/scc/light_client/provider"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestLightClient_NewLightClient_ReportsInvalidConfig(t *testing.T) {
	require := require.New(t)
	key := bls.NewPrivateKey()
	member := scc.Member{
		PublicKey:         key.PublicKey(),
		ProofOfPossession: key.GetProofOfPossession(),
		VotingPower:       1,
	}

	url := "http://localhost:4242"

	tests := map[string]Config{
		"emptyGenesisCommittee": {
			Genesis: scc.NewCommittee(),
		},
		"emptyStringProvider": {
			Provider: "",
			Genesis:  scc.NewCommittee(member),
		},
		"invalidProviderURL": {
			Provider: "not-a-url",
			Genesis:  scc.NewCommittee(member),
		},
		"emptyStateSource": {
			Provider:    url,
			Genesis:     scc.NewCommittee(member),
			StateSource: "",
		},
		"invalidStateSource": {
			Provider:    url,
			Genesis:     scc.NewCommittee(member),
			StateSource: "not-a-url",
		},
	}

	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			c, err := NewLightClient(config)
			require.Error(err)
			require.Nil(c)
		})
	}
}

func TestLightClient_NewLightClient_CreatesLightClientFromValidConfig(t *testing.T) {
	require := require.New(t)
	c, err := NewLightClient(testConfig())
	require.NoError(err)
	require.NotNil(c)
}

func TestLightClient_Close_ClosesProvider(t *testing.T) {
	// setup
	require := require.New(t)
	ctrl := gomock.NewController(t)
	prov := provider.NewMockProvider(ctrl)

	// expect
	prov.EXPECT().Close().Times(1)
	prov.EXPECT().GetBlockCertificates(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("provider is closed"))

	// build client
	c, err := NewLightClient(testConfig())
	require.NoError(err)
	c.provider = prov

	// check
	c.Close()
	_, err = c.Sync()
	require.ErrorContains(err, "provider is closed")
}

func TestLightClient_Sync_InitializesState(t *testing.T) {
	require := require.New(t)
	c, err := NewLightClient(testConfig())
	require.NoError(err)
	require.NotNil(c.state)
}

func TestLightClient_Sync_ReturnsErrorOnProviderFailure(t *testing.T) {
	//setup
	require := require.New(t)
	ctrl := gomock.NewController(t)
	prov := provider.NewMockProvider(ctrl)

	// expect
	errStr := "failed to get block certificates"
	prov.EXPECT().GetBlockCertificates(provider.LatestBlock, uint64(1)).
		Return(nil, fmt.Errorf("%v", errStr))

	// build client
	c, err := NewLightClient(testConfig())
	require.NoError(err)
	c.provider = prov

	// check
	_, err = c.Sync()
	require.ErrorContains(err, errStr)
}

func TestLightClient_Sync_ReturnsErrorOnStateSyncFailure(t *testing.T) {
	require := require.New(t)

	// setup mock provider
	ctrl := gomock.NewController(t)
	prov := provider.NewMockProvider(ctrl)

	// setup block certificate
	blockNumber := idx.Block(scc.BLOCKS_PER_PERIOD*1 + 42)
	blockCert := cert.NewCertificate(
		cert.NewBlockStatement(0, blockNumber, common.Hash{0x1}, common.Hash{}))

	// expect to return head
	prov.EXPECT().GetBlockCertificates(provider.LatestBlock, uint64(1)).
		Return([]cert.BlockCertificate{blockCert}, nil)

	// setup committee certificate
	committeeCert := cert.NewCertificate(cert.CommitteeStatement{Period: 1})

	// expect to return committee certificates that is not signed by genesis
	prov.EXPECT().
		GetCommitteeCertificates(scc.Period(1), gomock.Any()).
		Return([]cert.CommitteeCertificate{committeeCert}, nil)

	// create LightClient
	c, err := NewLightClient(testConfig())
	require.NoError(err)

	// set provider
	c.provider = prov

	// sync
	_, err = c.Sync()
	require.ErrorContains(err, "invalid committee")
}

func TestLightClientState_Sync_UpdatesStateToHead(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	prov := provider.NewMockProvider(ctrl)

	// Setup test data
	key := bls.NewPrivateKey()
	blockCert, blockNumber := setupBlockCertificate(t, key)
	committeeCert := setupCommitteeCertificate(t, key)

	// Mock provider calls
	mockProviderResponses(prov, blockCert, committeeCert)

	// Create and configure LightClient
	client, err := setupLightClient(prov, key)
	require.NoError(err)

	// Perform sync
	head, err := client.Sync()
	require.NoError(err)

	// Validate result
	require.Equal(blockNumber, head)
}

func TestLightClient_GetBalance_ReportsErrorOnSyncFailure(t *testing.T) {
	// setup
	require := require.New(t)
	ctrl := gomock.NewController(t)
	prov := provider.NewMockProvider(ctrl)

	// expect
	prov.EXPECT().GetBlockCertificates(provider.LatestBlock, uint64(1)).
		Return(nil, fmt.Errorf("failed to sync"))

	// build client
	c, err := NewLightClient(testConfig())
	require.NoError(err)
	c.provider = prov

	// check
	_, err = c.GetBalance(common.Address{0x01}, 1)
	require.ErrorContains(err, "failed to sync")
}

func TestLightClient_GetBalance_ReportsErrors(t *testing.T) {
	require := require.New(t)
	address := common.Address{0x01}

	// reports error from querier
	t.Run("querierError", func(t *testing.T) {
		client, querier := setupForTestSync(t)
		querier.EXPECT().GetAddressInfo(address, idx.Block(1)).
			Return(bq.ProofQuery{}, fmt.Errorf("some error"))
		_, err := client.GetBalance(address, 1)
		require.ErrorContains(err, "failed to get address info")
	})

	// reports mismatching state root
	t.Run("stateRootMismatch", func(t *testing.T) {
		client, querier := setupForTestSync(t)
		querier.EXPECT().GetAddressInfo(address, idx.Block(1)).
			Return(bq.ProofQuery{
				StorageHash: common.Hash{0x1},
				Balance:     uint256.NewInt(0),
			}, nil)
		_, err := client.GetBalance(address, 1)
		require.ErrorContains(err, "state root mismatch")
	})
}

func TestLightClient_GetBalance_ReturnsZeroWithNilBalance(t *testing.T) {
	require := require.New(t)
	client, querier := setupForTestSync(t)

	querier.EXPECT().GetAddressInfo(common.Address{0x01}, idx.Block(1)).
		Return(bq.ProofQuery{StorageHash: common.Hash{0x02}}, nil)

	balance, err := client.GetBalance(common.Address{0x01}, 1)
	require.NoError(err)
	require.Equal(uint64(0), balance)
}

func TestLightClient_GetBalance_ReturnsBalance(t *testing.T) {
	require := require.New(t)
	client, querier := setupForTestSync(t)

	querier.EXPECT().GetAddressInfo(common.Address{0x01}, idx.Block(1)).
		Return(bq.ProofQuery{
			StorageHash: common.Hash{0x02},
			Balance:     uint256.NewInt(42),
		}, nil)

	balance, err := client.GetBalance(common.Address{0x01}, 1)
	require.NoError(err)
	require.Equal(uint64(42), balance)
}

func TestLightClient_GetNonce_ReportsErrorOnFailure(t *testing.T) {
	require := require.New(t)
	client, querier := setupForTestSync(t)

	querier.EXPECT().GetAddressInfo(common.Address{0x01}, idx.Block(1)).
		Return(bq.ProofQuery{}, fmt.Errorf("some error"))

	_, err := client.GetNonce(common.Address{0x01}, 1)
	require.ErrorContains(err, "failed to get nonce")
}

func TestLightClient_GetNonce_ReportsNonce(t *testing.T) {
	require := require.New(t)
	client, querier := setupForTestSync(t)

	querier.EXPECT().GetAddressInfo(common.Address{0x01}, idx.Block(1)).
		Return(bq.ProofQuery{StorageHash: common.Hash{0x02}, Nonce: 42}, nil)

	nonce, err := client.GetNonce(common.Address{0x01}, 1)
	require.NoError(err)
	require.Equal(uint64(42), nonce)
}

/////////////////////////////////////////////////////
// Helper functions for testing
/////////////////////////////////////////////////////

// makeMember makes an scc.Member from a bls.PrivateKey
func makeMember(key bls.PrivateKey) scc.Member {
	return scc.Member{
		PublicKey:         key.PublicKey(),
		ProofOfPossession: key.GetProofOfPossession(),
		VotingPower:       1,
	}
}

// testConfig returns a valid Config for testing
func testConfig() Config {
	key := bls.NewPrivateKey()
	return Config{
		Provider:    "http://localhost:4242",
		Genesis:     scc.NewCommittee(makeMember(key)),
		StateSource: "http://localhost:4242",
	}
}

// setupBlockCertificate creates a block certificate for the second block of
// period 1 and signs it with the given key.
// Returns the block certificate and the block number.
func setupBlockCertificate(t *testing.T, key bls.PrivateKey) (cert.BlockCertificate, idx.Block) {
	blockNumber := idx.Block(scc.BLOCKS_PER_PERIOD*1 + 1)
	blockCert := cert.NewCertificate(
		cert.NewBlockStatement(0, blockNumber, common.Hash{0x1}, common.Hash{0x2}),
	)

	// Sign certificate
	err := blockCert.Add(scc.MemberId(0), cert.Sign(blockCert.Subject(), key))
	require.NoError(t, err)

	return blockCert, blockNumber
}

// setupCommitteeCertificate creates a committee certificate for period 1 and
// signs it with the given key.
// Returns the committee certificate.
func setupCommitteeCertificate(t *testing.T, key bls.PrivateKey) cert.CommitteeCertificate {
	member := makeMember(key)
	committeeCert := cert.NewCertificate(cert.CommitteeStatement{
		Period:    1,
		Committee: scc.NewCommittee(member),
	})

	// Sign certificate
	err := committeeCert.Add(scc.MemberId(0), cert.Sign(committeeCert.Subject(), key))
	require.NoError(t, err)

	return committeeCert
}

// mockProviderResponses mocks the provider responses for block and committee certificates
func mockProviderResponses(prov *provider.MockProvider, blockCert cert.BlockCertificate, committeeCert cert.CommitteeCertificate) {
	prov.EXPECT().
		GetBlockCertificates(provider.LatestBlock, uint64(1)).
		Return([]cert.BlockCertificate{blockCert}, nil)

	prov.EXPECT().
		GetCommitteeCertificates(scc.Period(1), uint64(1)).
		Return([]cert.CommitteeCertificate{committeeCert}, nil)
}

// setupLightClient creates a LightClient with a committee member based on
// the given key and a used the given provider for the client.
func setupLightClient(prov *provider.MockProvider, key bls.PrivateKey) (*LightClient, error) {
	member := makeMember(key)

	config := Config{
		Provider:    "http://localhost:4242",
		Genesis:     scc.NewCommittee(member),
		StateSource: "http://localhost:4242",
	}
	client, err := NewLightClient(config)
	if err != nil {
		return nil, err
	}

	client.provider = prov
	return client, nil
}

// setupForTestSync sets up a light client and the necessary mocks for a
// successful sync test.
// - sets up a mock provider that will return valid block/committee certificates
// - initializes a mock querier and sets it as the client's querier
// Returns the client and the querier.
func setupForTestSync(
	t *testing.T,
) (*LightClient, *bq.MockBlockQueryI) {
	ctrl := gomock.NewController(t)
	prov := provider.NewMockProvider(ctrl)
	querier := bq.NewMockBlockQueryI(ctrl)
	key := bls.NewPrivateKey()
	blockCert, _ := setupBlockCertificate(t, key)
	committeeCert := setupCommitteeCertificate(t, key)
	client, err := setupLightClient(prov, key)
	require.NoError(t, err)
	client.querier = querier
	mockProviderResponses(prov, blockCert, committeeCert)
	return client, querier
}
