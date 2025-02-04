package memory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTotalMemoryIsNotZero(t *testing.T) {
	require := require.New(t)
	require.Greater(TotalMemory(), uint64(0))
	require.Less(TotalMemory(), uint64(1<<50)) // 1 PiB (sanity check)
}

func TestFreeMemoryIsLessThanTotalMemory(t *testing.T) {
	require := require.New(t)
	require.Greater(FreeMemory(), uint64(0))
	require.Less(FreeMemory(), TotalMemory())
}

func TestPrint(t *testing.T) {
	t.Logf("Total memory: %d bytes = %d GiB", TotalMemory(), TotalMemory()/(1<<30))
	t.Logf("Free memory: %d bytes = %d GiB", FreeMemory(), FreeMemory()/(1<<30))
	t.Fail()
}
