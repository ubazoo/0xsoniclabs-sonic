package light_client

import (
	"fmt"
	"slices"

	"github.com/0xsoniclabs/consensus/inter/idx"
	"github.com/0xsoniclabs/sonic/scc"
	bq "github.com/0xsoniclabs/sonic/scc/light_client/block_query"
	lcs "github.com/0xsoniclabs/sonic/scc/light_client/light_client_state"
	"github.com/0xsoniclabs/sonic/scc/light_client/provider"
	"github.com/ethereum/go-ethereum/common"
)

// LightClient is the main entry point for the light client.
// It is responsible for managing the light client state and
// interacting with the provider.
type LightClient struct {
	provider provider.Provider
	state    *lcs.State
	querier  bq.BlockQueryI
}

// Config is used to configure the LightClient.
// It requires an url for the certificate providers and an initial committee.
type Config struct {
	Provider    string
	Genesis     scc.Committee
	StateSource string
}

// NewLightClient creates a new LightClient with the given config.
// returns an error if the config does not contain a valid provider or committee.
func NewLightClient(config Config) (*LightClient, error) {
	if err := config.Genesis.Validate(); err != nil {
		return nil, fmt.Errorf("invalid committee provided: %w", err)
	}
	p, err := provider.NewServerFromURL(config.Provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w\n", err)
	}
	b, err := bq.NewBlockQuery(config.StateSource)
	if err != nil {
		return nil, fmt.Errorf("failed to create block querier: %w\n", err)
	}
	return &LightClient{
		state: lcs.NewState(
			scc.NewCommittee(
				slices.Clone(config.Genesis.Members())...)),
		provider: p,
		querier:  b,
	}, nil
}

// Close closes the light client provider.
func (c *LightClient) Close() {
	c.provider.Close()
	c.querier.Close()
}

// Sync updates the light client state using certificates from the provider.
// This serves as the primary method for synchronizing the light client state
// with the network.
func (c *LightClient) Sync() (idx.Block, error) {
	return c.state.Sync(c.provider)
}

// GetBalance returns the balance of the given address at the given height.
// It returns an error if it fails to sync, fails to get the address info or
// the proof state root does not match the current state root.
func (c *LightClient) GetBalance(address common.Address, height idx.Block) (uint64, error) {
	proof, err := c.getProof(address, height)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}
	if proof.Balance == nil {
		return 0, nil
	}
	return proof.Balance.Uint64(), err
}

func (c *LightClient) GetNonce(address common.Address, height idx.Block) (uint64, error) {
	proof, err := c.getProof(address, height)
	if err != nil {
		return 0, fmt.Errorf("failed to get nonce: %w", err)
	}
	return uint64(proof.Nonce), err
}

// getProof is a helper function that attempts a sync and then asks the
// querier for the proof of the given address at the given height.
// It returns an error if it fails to sync, fails to get the address info or
// the proof state root does not match the current state root.
func (c *LightClient) getProof(address common.Address, height idx.Block) (bq.ProofQuery, error) {
	// always sync before querying
	_, err := c.Sync()
	if err != nil {
		return bq.ProofQuery{}, fmt.Errorf("failed to sync: %w", err)
	}
	proof, err := c.querier.GetAddressInfo(address, height)
	if err != nil {
		return bq.ProofQuery{}, fmt.Errorf("failed to get address info: %w", err)
	}
	// it is safe to ignore the hasSynced flag here because if there was an error
	// during sync, if would have exited in the first if.
	rootHash, _ := c.state.StateRoot()
	if proof.StorageHash != rootHash {
		return bq.ProofQuery{}, fmt.Errorf("state root mismatch")
	}
	return proof, nil
}
