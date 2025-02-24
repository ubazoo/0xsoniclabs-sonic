package cert

import (
	"fmt"

	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/utils/jsonhex"
)

// AggregatedSignatureJson is a JSON friendly representation of an AggregatedSignature.
type AggregatedSignatureJson[S Statement] struct {
	// signers:  BitSet[scc.MemberId]
	Signers jsonhex.Bytes `json:"signers" gencodec:"required"`
	// signature: bls.Signature
	Signature jsonhex.Bytes96 `json:"signature" gencodec:"required"`
}

// String returns the JSON string representation of the AggregatedSignatureJson.
func (a AggregatedSignatureJson[S]) String() string {
	return fmt.Sprintf(`{"signers":"%v","signature":"%v"}`, a.Signers, a.Signature)
}

// ToAggregatedSignature converts the AggregatedSignatureJson to an AggregatedSignature.
// Returns an error if the signature is invalid.
func (a AggregatedSignatureJson[S]) ToAggregatedSignature() (AggregatedSignature[S], error) {
	bitset := BitSet[scc.MemberId]{}
	bitset.mask = a.Signers[:]
	signature, err := bls.DeserializeSignature(a.Signature)
	if err != nil {
		return AggregatedSignature[S]{}, err
	}

	return AggregatedSignature[S]{
		signers:   bitset,
		signature: signature,
	}, nil
}

// AggregatedSignatureToJson converts an AggregatedSignature to an AggregatedSignatureJson.
func AggregatedSignatureToJson[S Statement](a AggregatedSignature[S]) AggregatedSignatureJson[S] {
	return AggregatedSignatureJson[S]{
		Signers:   a.signers.mask,
		Signature: jsonhex.Bytes96(a.signature.Serialize()),
	}
}
