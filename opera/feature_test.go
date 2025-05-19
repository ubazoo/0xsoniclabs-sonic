package opera

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFeature_String_ProducesHumanReadableRepresentation(t *testing.T) {
	tests := map[Feature]string{
		NilFeature:             "NilFeature",
		SingleProposerProtocol: "SingleProposerProtocol",
		Feature(327):           "Feature(327)",
	}

	for feature, expected := range tests {
		t.Run(expected, func(t *testing.T) {
			require.Equal(t, expected, feature.String())
		})
	}
}
