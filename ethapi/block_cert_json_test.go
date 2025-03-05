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
	"github.com/kaptinlin/jsonschema"
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

func TestBlockCertificate_ValidateJsonSchema(t *testing.T) {
	require := require.New(t)

	schemaJSON := `{
		"type": "array",
		"properties": {
			"chainId":{"type": "integer"},
			"number":{"type": "integer"},
			"hash":{"type": "string", "pattern": "^0x[0-9a-f]{64}"},
			"stateRoot":{"type": "string", "pattern": "^0x[0-9a-f]{64}"},
			"signers":{"type": "string","pattern": "^0x[0-9a-f]*$"},
			"signature":{"type": "string", "pattern": "^0x[0-9a-f]{192}$"}
		},
		"required":["chainId","number","hash","stateRoot","signers","signature"]
	}`

	// compile schema
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile([]byte(schemaJSON))
	require.NoError(err)

	// validate certificate with values
	testCert := makeTestBlockCert(t)
	data, err := json.Marshal(toJsonBlockCertificate(testCert))
	require.NoError(err)

	result := schema.Validate(data)
	res := result.IsValid()
	require.True(res)

	// validate empty certificate
	testCert = cert.BlockCertificate{}
	data, err = json.Marshal(toJsonBlockCertificate(testCert))
	require.NoError(err)

	result = schema.Validate(data)
	res = result.IsValid()
	require.True(res)
}

func TestBlockCertificate_EmptyCertificate_ContainsExpectedValues(t *testing.T) {
	require := require.New(t)
	emptyCert := cert.BlockCertificate{}
	data, err := json.Marshal(toJsonBlockCertificate(emptyCert))
	require.NoError(err)
	require.Contains(string(data), `"chainId":0`)
	require.Contains(string(data), `"number":0`)
	require.Contains(string(data), fmt.Sprintf(`"hash":"%v"`, common.Hash{}))
	require.Contains(string(data), fmt.Sprintf(`"stateRoot":"%v"`, common.Hash{}))
	require.Contains(string(data), `"signers":null`)
	require.Contains(string(data), fmt.Sprintf(`"signature":"%v"`, bls.Signature{}))
}

func TestBlockCertificate_NonEmptyCertificate_ContainsExpectedValues(t *testing.T) {
	require := require.New(t)
	testCert := makeTestBlockCert(t)
	agg := testCert.Signature()
	signers, err := json.Marshal(agg.Signers())
	require.NoError(err)

	data, err := json.Marshal(toJsonBlockCertificate(testCert))
	require.NoError(err)

	require.Contains(string(data), `"chainId":123`)
	require.Contains(string(data), `"number":456`)
	require.Contains(string(data), fmt.Sprintf(`"hash":"%v"`, common.Hash{0x1}))
	require.Contains(string(data), fmt.Sprintf(`"stateRoot":"%v"`, common.Hash{0x2}))
	require.Contains(string(data), fmt.Sprintf(`"signers":%v`, string(signers)))
	require.Contains(string(data), fmt.Sprintf(`"signature":"%v"`, agg.Signature()))
}

func makeTestBlockCert(t *testing.T) cert.BlockCertificate {
	key := bls.NewPrivateKey()
	sig := key.GetProofOfPossession()
	bitset := cert.BitSet[scc.MemberId]{}
	agg := cert.NewAggregatedSignature[cert.BlockStatement](
		bitset,
		sig,
	)
	err := agg.Add(1, cert.Signature[cert.BlockStatement]{Signature: key.Sign([]byte{1})})
	require.NoError(t, err)

	return cert.NewCertificateWithSignature(
		cert.NewBlockStatement(123, 456, common.Hash{0x1}, common.Hash{0x2}),
		agg,
	)
}
