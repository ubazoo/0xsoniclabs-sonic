package light_client

import (
	"fmt"
	"net/url"

	"github.com/0xsoniclabs/carmen/go/carmen"
	"github.com/0xsoniclabs/sonic/scc"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

// LightClient is the main entry point for the light client.
// It is responsible for managing the light client state and
// interacting with the provider.
type LightClient struct {
	provider provider
	state    state
}

// Config is used to configure the LightClient.
// It requires an url for the certificate provider and an initial committee.
type Config struct {
	Url     *url.URL
	Genesis scc.Committee
}

// NewLightClient creates a new LightClient with the given config.
// Returns an error if the config does not contain a valid provider url or committee.
func NewLightClient(config Config) (*LightClient, error) {
	if err := config.Genesis.Validate(); err != nil {
		return nil, fmt.Errorf("invalid committee provided: %w", err)
	}
	p, err := newServerFromURL(config.Url.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w\n", err)
	}
	return &LightClient{
		state:    *newState(config.Genesis),
		provider: p,
	}, nil
}

// Close closes the light client provider.
// Closing an already closed client has no effect
func (c *LightClient) Close() {
	c.provider.close()
}

// Sync updates the light client state using certificates from the provider.
// This serves as the primary method for synchronizing the light client state
// with the network.
func (c *LightClient) Sync() (idx.Block, error) {
	return c.state.sync(c.provider)
}

// GetBalance returns the balance of the given address.
// It returns an error if the balance could not be proven or there was any error
// in getting or verifying the proof.
func (c *LightClient) GetBalance(address common.Address) (*uint256.Int, error) {
	balance, err := getInfoFromProof(
		address,
		c,
		"balance",
		func(proof carmen.WitnessProof, address common.Address,
			rootHash common.Hash) (carmen.Amount, bool, error) {
			return proof.GetBalance(carmen.Hash(rootHash), carmen.Address(address))
		},
	)
	if err != nil {
		return nil, err
	}
	balanceInt := balance.Uint256()
	return &balanceInt, nil
}

// getAccountProof is a helper function that syncs and returns the proof
// for the given address.
// It returns an error if sync failed, the proof could not be obtained or verified.
func (c *LightClient) getAccountProof(address common.Address) (carmen.WitnessProof, error) {
	// always sync before querying
	_, err := c.Sync()
	if err != nil {
		return nil, fmt.Errorf("failed to sync: %w", err)
	}
	proof, err := c.provider.getAccountProof(address, LatestBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}
	if proof == nil {
		return nil, fmt.Errorf("failed to get account proof")
	}
	// verify the proof
	if !proof.IsValid() {
		return nil, fmt.Errorf("failed to verify proof: %w", err)
	}
	return proof, nil
}

// getInfoFromProof is a helper function takes an address and a function, which
// takes a proof, address and hash, and returns the desired information.
func getInfoFromProof[T any](address common.Address, c *LightClient, valueName string,
	f func(carmen.WitnessProof, common.Address, common.Hash) (T, bool, error)) (T, error) {
	proof, err := c.getAccountProof(address)
	var zeroValue T
	if err != nil {
		return zeroValue, fmt.Errorf("failed to get account info: %w", err)
	}
	// it is safe to ignore the hasSynced flag here because if there was an error
	// during sync, it would have triggered an early return.
	rootHash, _ := c.state.stateRoot()
	value, proven, err := f(proof, address, rootHash)
	if err != nil {
		return zeroValue, fmt.Errorf("failed to get %v from proof: %w", valueName, err)
	}
	if !proven {
		return zeroValue,
			fmt.Errorf("%v could not be proven from the proof and state root hash",
				valueName)
	}
	return value, err
}
