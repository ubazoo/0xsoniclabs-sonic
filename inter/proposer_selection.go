package inter

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/inter/pos"
)

// GetProposer returns the designated proposer for a given block number and attempt.
// The proposer is determined through deterministic sampling of validators
// proportional to the validator's stake.
func GetProposer(
	validators *pos.Validators,
	number idx.Block,
	attempt uint32,
) (idx.ValidatorID, error) {

	// TODO: consider excluding first proposer from second attempt mapping
	data := make([]byte, 0, 8+4)
	data = binary.BigEndian.AppendUint64(data, uint64(number))
	data = binary.BigEndian.AppendUint32(data, attempt)
	hash := sha256.Sum256(data)

	limit := new(big.Rat).Quo(
		new(big.Rat).SetInt(new(big.Int).SetBytes(hash[:])),
		new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 256)),
	)

	ids := validators.SortedIDs()
	weights := validators.SortedWeights()

	if len(ids) == 0 {
		return 0, fmt.Errorf("no validators")
	}
	totalWeight := validators.TotalWeight()
	if totalWeight == 0 {
		return 0, fmt.Errorf("no validators with weight")
	}

	curLimit := big.NewRat(0, 1)
	curLimit.Denom().SetInt64(int64(totalWeight))
	for i, id := range ids {
		weight := weights[i]
		curLimit.Num().Add(curLimit.Num(), big.NewInt(int64(weight)))
		if curLimit.Cmp(limit) >= 0 {
			return id, nil
		}
	}

	return ids[len(ids)-1], nil
}
