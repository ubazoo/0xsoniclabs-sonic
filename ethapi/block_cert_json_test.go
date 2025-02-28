package ethapi

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/scc/cert"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestBlockCertificateJson_ToBlockCertificate_ConvertsToHealthyCertificate(t *testing.T) {
	require := require.New(t)
	bitSet := cert.BitSet[scc.MemberId]{}
	bitSet.Add(1)
	newPrivateKey := bls.NewPrivateKey()
	sig := newPrivateKey.GetProofOfPossession()

	b := blockCertificateJson{
		ChainId:   123,
		Number:    456,
		Hash:      common.Hash{0x1},
		StateRoot: common.Hash{0x2},
		Signers:   bitSet,
		Signature: sig,
	}

	got := b.toCertificate()
	aggregatedSignature := cert.NewAggregatedSignature[cert.BlockStatement](
		b.Signers,
		b.Signature,
	)
	want := cert.NewCertificateWithSignature(cert.NewBlockStatement(
		b.ChainId,
		idx.Block(b.Number),
		b.Hash,
		b.StateRoot),
		aggregatedSignature)

	require.Equal(want, got)
}

func TestBlockCertificateJson_CanBeJsonEncodedAndDecoded(t *testing.T) {
	require := require.New(t)

	// setup
	private := bls.NewPrivateKey()
	agg := cert.AggregatedSignature[cert.BlockStatement]{}
	err := agg.Add(1, cert.Signature[cert.BlockStatement]{Signature: private.Sign([]byte{1})})
	require.NoError(err)
	c := cert.NewCertificateWithSignature(
		cert.NewBlockStatement(123, 456, common.Hash{0x1}, common.Hash{0x2}),
		agg,
	)

	// encode
	certJson := toJsonBlockCertificate(c)
	data, err := json.Marshal(certJson)
	require.NoError(err)

	// decode
	var decoded blockCertificateJson
	err = json.Unmarshal(data, &decoded)
	require.NoError(err)

	// check
	require.Equal(certJson, decoded)
	cert := decoded.toCertificate()
	require.Equal(c, cert)
}

func TestBlockCertificateToJson(t *testing.T) {
	require := require.New(t)
	bitset := cert.BitSet[scc.MemberId]{}
	bitset.Add(1)
	sig := bls.NewPrivateKey().GetProofOfPossession()
	cert := cert.NewCertificateWithSignature(
		cert.NewBlockStatement(123, 456, common.Hash{0x1}, common.Hash{0x2}),
		cert.NewAggregatedSignature[cert.BlockStatement](bitset, sig),
	)

	json := toJsonBlockCertificate(cert)
	require.Equal(uint64(123), json.ChainId)
	require.Equal(uint64(456), json.Number)
	require.Equal(common.Hash{0x1}, json.Hash)
	require.Equal(common.Hash{0x2}, json.StateRoot)
	require.Equal(bitset, json.Signers)
	require.Equal(sig, json.Signature)
}

func TestBlockCertificate_MarshallingProducesJsonFormatting(t *testing.T) {
	require := require.New(t)
	key := bls.NewPrivateKey()

	// empty case setup
	sig := key.GetProofOfPossession()

	// non-empty case setup
	bitset := cert.BitSet[scc.MemberId]{}
	agg := cert.NewAggregatedSignature[cert.BlockStatement](
		bitset,
		sig,
	)
	err := agg.Add(1, cert.Signature[cert.BlockStatement]{Signature: key.Sign([]byte{1})})
	require.NoError(err)

	tests := map[string]struct {
		cert cert.BlockCertificate
		want string
	}{
		"empty": {
			cert: cert.NewCertificateWithSignature(
				cert.NewBlockStatement(0, 0, common.Hash{}, common.Hash{}),
				cert.NewAggregatedSignature[cert.BlockStatement](
					cert.BitSet[scc.MemberId]{}, sig),
			),
			want: fmt.Sprintf(`{"chainId":0,"number":0,"hash":"%v","stateRoot":"%v","signers":null,"signature":"%v"}`,
				common.Hash{}, common.Hash{}, sig),
		},
		"non-empty": {
			cert: cert.NewCertificateWithSignature(
				cert.NewBlockStatement(123, 456, common.Hash{0x1}, common.Hash{0x2}),
				agg,
			),
			want: fmt.Sprintf(`{"chainId":123,"number":456,"hash":"%v","stateRoot":"%v","signers":"0x02","signature":"%v"}`,
				common.Hash{0x1}, common.Hash{0x2}, agg.Signature()),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			jsonCert := toJsonBlockCertificate(test.cert)
			data, err := json.Marshal(jsonCert)
			require.NoError(err)
			require.Equal(test.want, string(data))
		})
	}
}
