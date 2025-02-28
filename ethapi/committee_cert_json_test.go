package ethapi

import (
	"encoding/json"
	"fmt"
	"regexp"
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

func TestCommitteeCertificate_MarshallingProducesJsonFormatting(t *testing.T) {
	require := require.New(t)
	key := bls.NewPrivateKey()

	keyRegex := `"0x[0-9a-f]{96}"`
	signatureRegex := `"0x[0-9a-f]{192}"`
	memberRegex := fmt.Sprintf(`\[{"PublicKey":%v,"ProofOfPossession":%v,"VotingPower":\d+}+\]`,
		keyRegex, signatureRegex)
	signersRegex := `"(0x[0-9a-f]+)|null"`
	certRegexString := fmt.Sprintf(`{"chainId":\d+,"period":\d+,"members":%v,"signers":%v,"signature":%v}`,
		memberRegex, signersRegex, signatureRegex)
	certRegex := regexp.MustCompile(certRegexString)

	// empty case setup
	sig := key.GetProofOfPossession()
	// --

	// non-empty case setup
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
		sig,
	)
	err := agg.Add(1, cert.Signature[cert.CommitteeStatement]{Signature: key.Sign([]byte{1})})
	require.NoError(err)
	// --

	tests := map[string]struct {
		cert cert.CommitteeCertificate
		want string
	}{
		"empty cert": {
			cert: cert.NewCertificateWithSignature(
				cert.NewCommitteeStatement(0, 0, scc.NewCommittee()),
				cert.NewAggregatedSignature[cert.CommitteeStatement](
					cert.BitSet[scc.MemberId]{}, sig),
			),
			want: fmt.Sprintf(`{"chainId":0,"period":0,"members":null,"signers":"null","signature":"%v"}`,
				sig),
		},
		"non-empty cert": {
			cert: cert.NewCertificateWithSignature(
				cert.NewCommitteeStatement(123, 456, scc.NewCommittee(members...)),
				agg,
			),
			want: fmt.Sprintf(`{"chainId":123,"period":456,"members":[{"PublicKey":"%v","ProofOfPossession":"%v","VotingPower":%v}],"signers":"0x02","signature":"%v"}`,
				members[0].PublicKey, members[0].ProofOfPossession, members[0].VotingPower, agg.Signature()),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := toJsonCommitteeCertificate(test.cert)
			data, err := json.Marshal(got)
			require.NoError(err)
			require.Equal(test.want, string(data))
			require.Regexp(certRegex, string(data))
		})
	}
}
