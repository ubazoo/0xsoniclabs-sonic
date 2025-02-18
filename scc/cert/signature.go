package cert

import "github.com/0xsoniclabs/sonic/scc/bls"

type Signature[T Statement] struct {
	Signature bls.Signature
}
