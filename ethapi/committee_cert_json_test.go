package ethapi

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/scc/cert"
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

func TestCommitteeCertificate_JsonEncodingMatchesExpectedFormat(t *testing.T) {
	tests := map[string]struct {
		cert cert.CommitteeCertificate
	}{
		"empty cert":     {cert: makeEmptyCommitteeCert()},
		"non-empty cert": {makeTestCommitteeCert(t)},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			validateCommitteeCertJsonFormat(t, test.cert)
		})
	}
}

func validateCommitteeCertJsonFormat(t *testing.T, cert cert.CommitteeCertificate) {
	keyRegex := `("0x[0-9a-f]{96}")`
	signatureRegex := `("0x[0-9a-f]{192}")`
	member := fmt.Sprintf(`(\[{"PublicKey":%v,"ProofOfPossession":%v,"VotingPower":\d+}+\]|null)`,
		keyRegex, signatureRegex)

	tests := map[string]struct {
		regex string
	}{
		"chainId":   {regex: `"chainId":\d+`},
		"period":    {regex: `"period":\d+`},
		"member":    {regex: member},
		"signers":   {regex: `"signers":("0x[0-9a-f]+"|null)`},
		"signature": {regex: `"signature":` + signatureRegex},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			data, err := json.Marshal(toJsonCommitteeCertificate(cert))
			require.NoError(err)
			require.Regexp(test.regex, string(data))
		})
	}
}

func TestCommitteeCertificate_ContainsExpectedValues_EmptyCertificate(t *testing.T) {
	require := require.New(t)
	cert := makeEmptyCommitteeCert()
	data, err := json.Marshal(toJsonCommitteeCertificate(cert))
	require.NoError(err)

	require.Contains(string(data), `"chainId":0`)
	require.Contains(string(data), `"period":0`)
	require.Contains(string(data), `"members":null`)
	require.Contains(string(data), `"signers":null`)
	require.Contains(string(data), `"signature":"0x`)
}

func TestCommitteeCertificate_ContainsExpectedValues_NonEmptyCertificate(t *testing.T) {
	require := require.New(t)
	cert := makeTestCommitteeCert(t)
	member := cert.Subject().Committee.Members()[0]
	data, err := json.Marshal(toJsonCommitteeCertificate(cert))
	require.NoError(err)

	memberString := fmt.Sprintf(`{"PublicKey":"%v","ProofOfPossession":"%v","VotingPower":%v}`,
		member.PublicKey, member.ProofOfPossession, member.VotingPower)

	require.Contains(string(data), `"chainId":123`)
	require.Contains(string(data), `"period":456`)
	require.Contains(string(data), memberString)
	require.Contains(string(data), `"signers":"0x`)
	require.Contains(string(data), `"signature":"0x`)
}

func makeEmptyCommitteeCert() cert.CommitteeCertificate {
	return cert.NewCertificateWithSignature(
		cert.NewCommitteeStatement(0, 0, scc.NewCommittee()),
		cert.NewAggregatedSignature[cert.CommitteeStatement](
			cert.BitSet[scc.MemberId]{}, bls.Signature{}),
	)
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
