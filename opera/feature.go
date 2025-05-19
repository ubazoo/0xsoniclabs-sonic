package opera

//go:generate stringer -type=Feature

// Feature is an enumeration of features that can be enabled or disabled in
// the client. Feature implementations should use this enumeration to determine
// if a feature is enabled or disabled in the current configuration.
//
// Unlike hard forks, features can be enabled or disabled over the course of the
// network's evolution. However, this is not a general requirement. Individual
// features should be documented to indicate if they can be disabled safely.
type Feature uint16

// A list of feature flags. The numeric values of these flags are equivalent to
// the feature's identity and must not be changed after the feature is enabled
// on the network for the first time.
const (
	// NilFeature is a named constant for the default value of a feature flag
	// not to be used by any feature.
	NilFeature Feature = 0x0

	// SingleProposerProtocol is a feature flag facilitating the transition to
	// an operation mode where the full list of transactions to be included in
	// a block is determined by a single proposer. This feature is introduced
	// as part of v2.1 and must only be enabled on networks where all critical
	// components are updated to support it.
	//
	// The main intention of this feature is to facilitate the experimentation
	// with new protocols in various environments.
	//
	// This feature can be disabled at any time, leading to a return to Sonic's
	// classic block formation protocol.
	SingleProposerProtocol Feature = 0x1
)
