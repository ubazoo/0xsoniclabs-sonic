package opera

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
)

// FeatureSet is a collection of features used to track the status of features
// in the client. Feature sets support forward-compatible serialization, meaning
// that new features can be added without breaking existing serialization. A
// client receiving an update for unknown features can track those updates,
// unaware of the enabled features. However, when being replaced by an updated
// version of the client, the data stored in the database contains the enabled
// features, which may not be recognized by the new version.
//
// This facilitates the roll-out of non-consensus critical features to networks
// with partially updated clients. As the remaining clients are updated, they
// will see the enabled-status of new features and start using them.
type FeatureSet struct {
	mask []byte
}

// Clone creates a deep copy of the FeatureSet.
func (fs FeatureSet) Clone() FeatureSet {
	return FeatureSet{
		mask: bytes.Clone(fs.mask),
	}
}

// IsEmpty checks if the feature set is empty. An empty feature set has no
// features enabled.
func (fs FeatureSet) IsEmpty() bool {
	return len(fs.mask) == 0
}

// IsEnabled checks if a feature is enabled in the feature set.
func (fs FeatureSet) IsEnabled(feature Feature) bool {
	word := int(feature) / 8
	bit := int(feature) % 8
	if word >= len(fs.mask) {
		return false
	}
	return fs.mask[word]&(1<<bit) != 0
}

// Enable enables a feature in the feature set. If the feature is already
// enabled, this function does nothing.
func (fs *FeatureSet) Enable(feature Feature) {
	word := int(feature) / 8
	if word >= len(fs.mask) {
		newFeatures := make([]byte, word+1)
		copy(newFeatures, fs.mask)
		fs.mask = newFeatures
	}
	bit := int(feature) % 8
	fs.mask[word] |= 1 << bit
}

// Disable disables a feature in the feature set. If the feature is already
// disabled, this function does nothing.
func (fs *FeatureSet) Disable(feature Feature) {
	word := int(feature) / 8
	if word >= len(fs.mask) {
		return
	}
	bit := int(feature) % 8
	fs.mask[word] &^= 1 << bit
	fs.shrink()
}

// shrink reduces the size of the internal bit-mask slice to a minimum.
func (fs *FeatureSet) shrink() {
	// Remove trailing zero bytes
	for i := len(fs.mask) - 1; i >= 0; i-- {
		if fs.mask[i] != 0 {
			break
		}
		fs.mask = fs.mask[:i]
	}
}

// String produces a human-readable representation of the feature set.
func (fs FeatureSet) String() string {
	var features []string
	for i := range fs.mask {
		for j := 0; j < 8; j++ {
			if fs.mask[i]&(1<<j) != 0 {
				features = append(features, Feature(i*8+j).String())
			}
		}
	}
	return "{" + strings.Join(features, ", ") + "}"
}

// --- RLP serialization ---

// EncodeRLP supports serialization of feature sets using RLP encoding.
func (fs FeatureSet) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, fs.mask)
}

// DecodeRLP supports deserialization of feature sets using RLP encoding.
func (fs *FeatureSet) DecodeRLP(s *rlp.Stream) error {
	var data []byte
	if err := s.Decode(&data); err != nil {
		return err
	}
	fs.mask = bytes.Clone(data)
	fs.shrink()
	return nil
}

// --- JSON serialization ---

// MarshalJSON encodes the feature set as a hex-encoded string.
func (fs FeatureSet) MarshalJSON() ([]byte, error) {
	encoded := fmt.Sprintf("0x%x", fs.mask)
	return json.Marshal(encoded)
}

// UnmarshalJSON decodes a hex-encoded string into the feature set.
func (fs *FeatureSet) UnmarshalJSON(data []byte) error {
	var encoded string
	if err := json.Unmarshal(data, &encoded); err != nil {
		return err
	}
	if !strings.HasPrefix(encoded, "0x") {
		return fmt.Errorf("invalid feature set encoding: %s", encoded)
	}
	encoded = strings.TrimPrefix(encoded, "0x")

	var mask []byte
	mask, err := hex.AppendDecode(nil, []byte(encoded))
	if err != nil {
		return err
	}
	fs.mask = bytes.Clone(mask)
	fs.shrink()
	return nil
}
