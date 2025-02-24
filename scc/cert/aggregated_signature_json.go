package cert

import (
	"fmt"

	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/scc/cert/serialization"
)

type AggregatedSignatureJson[S Statement] struct {
	// signers: []MemberId
	Signers []byte `json:"signers" gencodec:"required"`
	// signature: Signature
	Signature serialization.HexBytes96 `json:"signature" gencodec:"required"`
}

func (a AggregatedSignatureJson[S]) String() string {
	return fmt.Sprintf(`{"signers":%v,"signature":"%v"}`, a.Signers, a.Signature)
}

func (a AggregatedSignatureJson[S]) ToAggregatedSignature() (AggregatedSignature[S], error) {
	bitset := BitSet[scc.MemberId]{}
	bitset.Deserialize(a.Signers)
	signature, err := bls.DeserializeSignature(a.Signature)
	if err != nil {
		return AggregatedSignature[S]{}, err
	}

	return AggregatedSignature[S]{
		signers:   bitset,
		signature: signature,
	}, nil
}

func AggregatedSignatureToJson[S Statement](a *AggregatedSignature[S]) (AggregatedSignatureJson[S], error) {
	stringSigners := a.signers.Serialize()
	signatures := a.signature.Serialize()
	return AggregatedSignatureJson[S]{
		Signers:   stringSigners,
		Signature: serialization.HexBytes96(signatures),
	}, nil
}
