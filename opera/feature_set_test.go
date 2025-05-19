package opera

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

func TestFeatureSet_DefaultIsEmpty(t *testing.T) {
	set := FeatureSet{}
	require.True(t, set.IsEmpty())
	require.Equal(t, 0, len(set.mask))
	require.Equal(t, "{}", set.String())
}

func TestFeatureSet_Clone_CreatesIndependentCopy(t *testing.T) {
	require := require.New(t)

	original := FeatureSet{}
	original.Enable(1)
	original.Enable(7)
	original.Enable(8)
	original.Enable(127)

	clone := original.Clone()
	require.Equal(original, clone)

	clone.Disable(7)
	require.NotEqual(original, clone)
	require.True(original.IsEnabled(7))
	require.False(clone.IsEnabled(7))

	clone.Enable(7)
	require.Equal(original, clone)
}

func TestFeatureSet_IsEmpty_TrueIfNoFeatureIsEnabled(t *testing.T) {
	require := require.New(t)

	set := FeatureSet{}
	require.True(set.IsEmpty())

	set.Enable(0)
	require.False(set.IsEmpty())

	set.Disable(0)
	require.True(set.IsEmpty())
}

func TestFeatureSet_IsEnabled_SupportsFeaturesOutOfRange(t *testing.T) {
	require := require.New(t)

	set := FeatureSet{}
	for i := range Feature(20) {
		for j := range Feature(20) {
			require.Equal(j < i, set.IsEnabled(j))
		}
		set.Enable(i)
	}
}

func TestFeatureSet_EnableAndDisable_TogglesFeatureState(t *testing.T) {
	require := require.New(t)

	features := []Feature{
		SingleProposerProtocol,
		Feature(1),
		Feature(7),
		Feature(8),
		Feature(127),
	}

	set := FeatureSet{}

	for _, feature := range features {
		require.False(set.IsEnabled(feature))
		set.Enable(feature)
		require.True(set.IsEnabled(feature))
		set.Disable(feature)
		require.False(set.IsEnabled(feature))
	}
}

func TestFeatureSet_EnableAndDisable_DoNotAffectOtherFeatures(t *testing.T) {
	N := 37
	for i := range Feature(N) {
		for j := range Feature(N) {
			if i == j {
				continue
			}
			set := FeatureSet{}
			require.False(t, set.IsEnabled(i))
			require.False(t, set.IsEnabled(j))
			set.Enable(i)
			require.True(t, set.IsEnabled(i))
			require.False(t, set.IsEnabled(j))
			set.Enable(j)
			require.True(t, set.IsEnabled(i))
			require.True(t, set.IsEnabled(j))
			set.Disable(i)
			require.False(t, set.IsEnabled(i))
			require.True(t, set.IsEnabled(j))
			set.Disable(j)
			require.False(t, set.IsEnabled(i))
			require.False(t, set.IsEnabled(j))
		}
	}
}

func TestFeatureSet_Disable_IgnoresDisabledFeatures(t *testing.T) {
	require := require.New(t)

	features := []Feature{
		SingleProposerProtocol,
		Feature(1),
		Feature(7),
		Feature(8),
		Feature(127),
	}

	set := FeatureSet{}

	for _, feature := range features {
		set.Disable(feature)
		require.False(set.IsEnabled(feature))
	}
}

func TestFeatureSet_Disable_ShrinksFeatureMask(t *testing.T) {
	require := require.New(t)
	set := FeatureSet{}
	set.Enable(7)
	require.Len(set.mask, 1)
	set.Enable(12)
	require.Len(set.mask, 2)
	set.Enable(124)
	require.Len(set.mask, 16)
	set.Disable(124)
	require.Len(set.mask, 2)
	set.Enable(61)
	require.Len(set.mask, 8)
	set.Disable(12)
	require.Len(set.mask, 8)
	set.Disable(61)
	require.Len(set.mask, 1)
	set.Disable(7)
	require.Len(set.mask, 0)
}

func TestFeatureSet_String_PrintsFeatureNamesSorted(t *testing.T) {
	require := require.New(t)

	features := []Feature{1, 2, 3, 7, 8, 127, 128, 129}
	set := FeatureSet{}
	var names []string
	for i := range features {
		names = append(names, features[i].String())
		set.Enable(features[i])
	}

	require.Equal("{"+strings.Join(names, ", ")+"}", set.String())
}

func TestFeatureSet_RLP_EncodingAndDecoding(t *testing.T) {
	require := require.New(t)

	set := FeatureSet{}
	for _, feature := range []Feature{1, 2, 3, 7, 8, 127, 128, 129} {
		set.Enable(feature)
	}

	data, err := rlp.EncodeToBytes(set)
	require.NoError(err)

	restored := FeatureSet{}
	restored.Enable(9)

	err = rlp.DecodeBytes(data, &restored)
	require.NoError(err)

	for feature := range Feature(250) {
		require.Equal(set.IsEnabled(feature), restored.IsEnabled(feature))
	}
}

func TestFeatureSet_DecodeRLP_DetectsInvalidEncoding(t *testing.T) {
	tests := map[string][]byte{
		"empty":   {},
		"invalid": {0x01, 0x02, 0x03},
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			restored := FeatureSet{}
			require.Error(t, rlp.DecodeBytes(data, &restored))
		})
	}
}

func TestFeatureSet_JSON_MarshalAndUnmarshal(t *testing.T) {
	require := require.New(t)

	set := FeatureSet{}
	for _, feature := range []Feature{1, 2, 3, 7, 8, 127, 128, 129} {
		set.Enable(feature)
	}

	data, err := json.Marshal(set)
	require.NoError(err)

	restored := FeatureSet{}
	restored.Enable(9)

	err = json.Unmarshal(data, &restored)
	require.NoError(err)

	for feature := range Feature(250) {
		require.Equal(set.IsEnabled(feature), restored.IsEnabled(feature))
	}
}

func TestFeatureSet_MarshalJSON_ProducesReadableEncoding(t *testing.T) {
	require := require.New(t)

	set := FeatureSet{}
	for _, feature := range []Feature{2, 3, 8} {
		set.Enable(feature)
	}

	data, err := json.Marshal(set)
	require.NoError(err)

	got := string(data)
	want := "\"0x0c01\""
	require.Equal(want, got, "JSON encoding does not match expected format")
}

func TestFeatureSet_UnmarshalJSON_DetectsEncodingIssues(t *testing.T) {
	tests := map[string]string{
		"empty":           "",
		"invalid prefix":  "\"0y01\"",
		"odd length":      "\"0x012\"",
		"unexpected json": "{}",
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			restored := FeatureSet{}
			require.Error(t, json.Unmarshal([]byte(data), &restored))
		})
	}
}
