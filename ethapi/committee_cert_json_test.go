package ethapi

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/scc/cert"
	"github.com/kaptinlin/jsonschema"
	"github.com/stretchr/testify/require"
)

func TestCommitteeCertificateJson_ToCommitteeCertificate_ConvertsToHealthyCertificate(t *testing.T) {
	require := require.New(t)
	bitSet := cert.BitSet[scc.MemberId]{}
	bitSet.Add(1)
	sig := bls.Signature{}

	c := committeeCertificateJson{
		ChainId:   123,
		Period:    456,
		Members:   []scc.Member{},
		Signers:   bitSet,
		Signature: sig,
	}

	got := c.toCertificate()
	aggregatedSignature := cert.NewAggregatedSignature[cert.CommitteeStatement](
		c.Signers,
		c.Signature,
	)
	want := cert.NewCertificateWithSignature(cert.NewCommitteeStatement(
		c.ChainId,
		scc.Period(c.Period),
		scc.NewCommittee(c.Members...)),
		aggregatedSignature)

	require.Equal(want, got)
}

func TestCommitteeCertificateJson_CanBeJsonEncodedAndDecoded(t *testing.T) {
	require := require.New(t)

	// setup
	private := bls.NewPrivateKey()
	member := scc.Member{
		PublicKey:         private.PublicKey(),
		ProofOfPossession: private.GetProofOfPossession(),
		VotingPower:       1,
	}
	agg := cert.AggregatedSignature[cert.CommitteeStatement]{}
	err := agg.Add(1, cert.Signature[cert.CommitteeStatement]{Signature: private.Sign([]byte{1})})
	require.NoError(err)
	c := cert.NewCertificateWithSignature(
		cert.NewCommitteeStatement(123, 456, scc.NewCommittee(member)),
		agg,
	)

	// encode
	certJson := toJsonCommitteeCertificate(c)
	data, err := json.Marshal(certJson)
	require.NoError(err)

	// decode
	var certJson2 committeeCertificateJson
	err = json.Unmarshal(data, &certJson2)
	require.NoError(err)

	// check
	cert := certJson2.toCertificate()
	require.Equal(c, cert)
}

func TestCommitteeCertificateToJson(t *testing.T) {
	require := require.New(t)
	bitSet := cert.BitSet[scc.MemberId]{}
	bitSet.Add(1)
	sig := bls.Signature{}

	aggregatedSignature := cert.NewAggregatedSignature[cert.CommitteeStatement](
		bitSet,
		sig,
	)
	c := cert.NewCertificateWithSignature(
		cert.NewCommitteeStatement(123, 456, scc.NewCommittee()),
		aggregatedSignature,
	)

	want := committeeCertificateJson{
		ChainId:   123,
		Period:    456,
		Members:   []scc.Member(nil),
		Signers:   bitSet,
		Signature: sig,
	}
	got := toJsonCommitteeCertificate(c)
	require.Equal(want, got)
}

func TestCommitteeCertificate_ValidateJsonSchema(t *testing.T) {
	require := require.New(t)

	schemaJSON := `{
		"type": "array",
		"properties": {
			"chainId": {"type": "integer"},
			"period": {"type": "integer"},
			"members": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"PublicKey": {"type": "string","pattern": "^0x[0-9a-f]{96}$"},
						"ProofOfPossession": {"type": "string","pattern": "^0x[0-9a-f]{192}$"},
						"VotingPower": {"type": "integer"}
					},
					"required": ["PublicKey", "ProofOfPossession", "VotingPower"]
				}
			},
			"signers": {"type": "string","pattern": "^0x[0-9a-f]*$"},
			"signature": {"type": "string","pattern": "^0x[0-9a-f]{192}$"}
		},
		"required": ["chainId", "period", "members", "signers", "signature"]
	}`

	// compile schema
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile([]byte(schemaJSON))
	require.NoError(err)

	// validate certificate with values
	testCert := makeTestCommitteeCert(t)
	data, err := json.Marshal(toJsonCommitteeCertificate(testCert))
	require.NoError(err)

	result := schema.Validate(data)
	res := result.IsValid()
	require.True(res)

	// validate empty certificate
	testCert = cert.CommitteeCertificate{}
	data, err = json.Marshal(toJsonCommitteeCertificate(testCert))
	require.NoError(err)

	result = schema.Validate(data)
	res = result.IsValid()
	require.True(res)
}

func TestCommitteeCertificate_EmptyCertificate_ContainsExpectedValues(t *testing.T) {
	require := require.New(t)
	cert := cert.CommitteeCertificate{}
	data, err := json.Marshal(toJsonCommitteeCertificate(cert))
	require.NoError(err)

	require.Contains(string(data), `"chainId":0`)
	require.Contains(string(data), `"period":0`)
	require.Contains(string(data), `"members":null`)
	require.Contains(string(data), `"signers":null`)
	require.Contains(string(data), fmt.Sprintf(`"signature":"%v"`, bls.Signature{}))
}

func TestCommitteeCertificate_NonEmptyCertificate_ContainsExpectedValues(t *testing.T) {
	require := require.New(t)
	cert := makeTestCommitteeCert(t)
	member := cert.Subject().Committee.Members()[0]
	agg := cert.Signature()
	signers, err := json.Marshal(agg.Signers())
	require.NoError(err)

	data, err := json.Marshal(toJsonCommitteeCertificate(cert))
	require.NoError(err)

	memberString := fmt.Sprintf(`"members":[{"PublicKey":"%v","ProofOfPossession":"%v","VotingPower":%v}]`,
		member.PublicKey, member.ProofOfPossession, member.VotingPower)

	require.Contains(string(data), `"chainId":123`)
	require.Contains(string(data), `"period":456`)
	require.Contains(string(data), memberString)
	require.Contains(string(data), fmt.Sprintf(`"signers":%v`, string(signers)))
	require.Contains(string(data), fmt.Sprintf(`"signature":"%v"`, agg.Signature()))
}

func makeTestCommitteeCert(t *testing.T) cert.CommitteeCertificate {
	key := bls.NewPrivateKey()
	members := []scc.Member{
		{
			PublicKey:         key.PublicKey(),
			ProofOfPossession: key.GetProofOfPossession(),
			VotingPower:       1,
		},
	}
	bitset := cert.BitSet[scc.MemberId]{}
	agg := cert.NewAggregatedSignature[cert.CommitteeStatement](
		bitset,
		key.GetProofOfPossession(),
	)
	err := agg.Add(1, cert.Signature[cert.CommitteeStatement]{Signature: key.Sign([]byte{1})})
	require.NoError(t, err)

	return cert.NewCertificateWithSignature(
		cert.NewCommitteeStatement(123, 456, scc.NewCommittee(members...)),
		agg,
	)
}
