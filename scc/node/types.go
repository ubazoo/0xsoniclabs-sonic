package node

import "github.com/0xsoniclabs/sonic/scc/cert"

type CommitteeCertificate = cert.Certificate[cert.CommitteeStatement]
type BlockCertificate = cert.Certificate[cert.BlockStatement]
