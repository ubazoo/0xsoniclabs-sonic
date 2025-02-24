package cert

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/utils/jsonhex"
	"github.com/stretchr/testify/require"
)

func TestAggregatedSignatureJson_String(t *testing.T) {
	a := AggregatedSignatureJson[testStatement]{}
	zeroHex := jsonhex.Bytes96{}
	expected := fmt.Sprintf(`{"signers":"null","signature":"%v"}`, zeroHex.String())
	require.Equal(t, expected, a.String())
}

func TestAggregatedSignatureJson_ToAggregatedSignature_InvalidSignature(t *testing.T) {
	a := AggregatedSignatureJson[testStatement]{
		Signers:   []byte{},
		Signature: jsonhex.Bytes96{},
	}
	_, err := a.ToAggregatedSignature()
	require.Error(t, err)
}

func TestAggregatedSignatureJson_ToAggregatedSignature_ValidSignature(t *testing.T) {
	newSign := bls.Signature{}
	a := AggregatedSignatureJson[testStatement]{
		Signers:   []byte{0x01},
		Signature: jsonhex.Bytes96(newSign.Serialize()),
	}
	_, err := a.ToAggregatedSignature()
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
	require.NoError(agg.Add(123, sig2))

	json := AggregatedSignatureToJson(agg)
	str := json.String()
	wantString := fmt.Sprintf(`{"signers":"%v","signature":"%v"}`,
		jsonhex.Bytes(agg.signers.mask).String(),
		jsonhex.Bytes96(agg.signature.Serialize()))
	require.Equal(wantString, str)

	agg2, err := json.ToAggregatedSignature()
	require.NoError(err)

	require.Equal(agg, agg2)
}
