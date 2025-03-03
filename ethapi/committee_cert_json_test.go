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

type committeeCertValues struct {
	chainID   uint64
	period    uint64
	members   []scc.Member
	signers   cert.BitSet[scc.MemberId]
	signature bls.Signature
}

func checkCommitteeCertRegexFormat(t *testing.T, cert cert.CommitteeCertificate, want committeeCertValues) {
	keyRegexString := `("0x[0-9a-f]{96}")`
	signatureRegexString := `("0x[0-9a-f]{192}")`
	memberRegexString := fmt.Sprintf(`(\[{"PublicKey":%v,"ProofOfPossession":%v,"VotingPower":\d+}+\]|null)`,
		keyRegexString, signatureRegexString)
	signersRegexString := `("0x[0-9a-f]+"|null)`
	certRegexString := fmt.Sprintf(`{"chainId":(\d+),"period":(\d+),"members":%v,"signers":%v,"signature":%v}`,
		memberRegexString, signersRegexString, signatureRegexString)

	tests := map[string]struct {
		regex *regexp.Regexp
	}{
		"signatureRegex": {
			regex: regexp.MustCompile(signatureRegexString),
		},
		"memberRegex": {
			regex: regexp.MustCompile(memberRegexString),
		},
		"signersRegex": {
			regex: regexp.MustCompile(signersRegexString),
		},
		"certRegex": {
			regex: regexp.MustCompile(certRegexString),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			data, err := json.Marshal(toJsonCommitteeCertificate(cert))
			require.NoError(err)
			require.True(test.regex.MatchString(string(data)))
		})
	}
}

func checkCommitteeCertFormat(t *testing.T, cert cert.CommitteeCertificate, want committeeCertValues) {
	signers, err := json.Marshal(want.signers)
	require.NoError(t, err)
	members, err := json.Marshal(want.members)
	require.NoError(t, err)
	wantCert := fmt.Sprintf(`{"chainId":%d,"period":%d,"members":%v,"signers":%v,"signature":"%v"}`,
		want.chainID, want.period, string(members), string(signers), want.signature)

	data, err := json.Marshal(toJsonCommitteeCertificate(cert))
	require.NoError(t, err)
	require.Equal(t, wantCert, string(data))
}

func TestCommitteeCertificate_MarshalingProducesExpectedJsonFormatting(t *testing.T) {
	require := require.New(t)
	key := bls.NewPrivateKey()

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
		want committeeCertValues
	}{
		"empty cert": {
			cert: cert.NewCertificateWithSignature(
				cert.NewCommitteeStatement(0, 0, scc.NewCommittee()),
				cert.NewAggregatedSignature[cert.CommitteeStatement](
					cert.BitSet[scc.MemberId]{}, sig),
			),
			want: committeeCertValues{
				chainID:   0,
				period:    0,
				members:   nil,
				signers:   cert.BitSet[scc.MemberId]{},
				signature: sig,
			},
		},
		"non-empty cert": {
			cert: cert.NewCertificateWithSignature(
				cert.NewCommitteeStatement(123, 456, scc.NewCommittee(members...)),
				agg,
			),
			want: committeeCertValues{
				chainID:   123,
				period:    456,
				members:   members,
				signers:   agg.Signers(),
				signature: agg.Signature(),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// check regex format
			checkCommitteeCertRegexFormat(t, test.cert, test.want)
			// check exact values are as expected
			checkCommitteeCertFormat(t, test.cert, test.want)
		})
	}
}
