package cert

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/scc/cert/serialization"
	"github.com/stretchr/testify/require"
)

func TestAggregatedSignatureJson_String(t *testing.T) {
	a := AggregatedSignatureJson[testStatement]{}
	zeroHex := serialization.HexBytes96{}
	expected := fmt.Sprintf(`{"signers":[],"signature":"%v"}`, zeroHex.String())
	require.Equal(t, expected, a.String())
}

func TestAggregatedSignatureJson_ToAggregatedSignature_InvalidSignature(t *testing.T) {
	a := AggregatedSignatureJson[testStatement]{
		Signers:   []byte{},
		Signature: serialization.HexBytes96{},
	}
	_, err := a.ToAggregatedSignature()
	require.Error(t, err)
}

func TestAggregatedSignatureJson_ToAggregatedSignature_ValidSignature(t *testing.T) {
	newSign := bls.Signature{}
	a := AggregatedSignatureJson[testStatement]{
		Signers:   []byte{0x01},
		Signature: serialization.HexBytes96(newSign.Serialize()),
	}
	_, err := a.ToAggregatedSignature()
	require.NoError(t, err)

}

func TestAggregatedSignatureToJson(t *testing.T) {
	newSign := bls.Signature{}
	a := AggregatedSignature[testStatement]{
		signers:   BitSet[scc.MemberId]{},
		signature: newSign,
	}
	_, err := AggregatedSignatureToJson(&a)
	require.NoError(t, err)
}

func TestAggregatedSignatureJson_EndToEnd(t *testing.T) {
	require := require.New(t)
	key1 := bls.NewPrivateKey()
	key2 := bls.NewPrivateKey()

	stmt := testStatement(1)
	sig1 := Sign(stmt, key1)
	sig2 := Sign(stmt, key2)

	agg := AggregatedSignature[testStatement]{}
	require.NoError(agg.Add(1, sig1))
	require.NoError(agg.Add(2, sig2))

	json, err := AggregatedSignatureToJson(&agg)
	require.NoError(err)

	agg2, err := json.ToAggregatedSignature()
	require.NoError(err)

	require.Equal(agg, agg2)
}
