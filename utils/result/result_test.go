package result

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResult_New_ProducesASuccessfulResult(t *testing.T) {
	res := New[int](42)
	require.False(t, res.IsError())
	v, err := res.Unwrap()
	require.Equal(t, 42, v)
	require.Nil(t, err)
}

func TestResult_Error_ProducesAFailedResult(t *testing.T) {
	require.True(t, Error[int](fmt.Errorf("fail")).IsError())
}

func TestResult_Error_NilResultsInZeroValue(t *testing.T) {
	res := Error[int](nil)
	require.False(t, res.IsError())
	v, err := res.Unwrap()
	require.Zero(t, v)
	require.Nil(t, err)
}
