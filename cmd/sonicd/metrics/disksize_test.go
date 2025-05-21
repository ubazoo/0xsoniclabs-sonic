//go:build goexperiment.synctest

package metrics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/stretchr/testify/require"
)

func TestMeasureDbDir_LogsDBDirSizeEveryMinute(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create some data to write to the test file
	testData := []byte("test data")
	lenTestData := len(testData)

	// Create a test file in the temporary directory
	testFile1 := filepath.Join(tempDir, "file1")
	f, err := os.OpenFile(testFile1, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	require.NoError(t, err)

	gaugeName := "testGauge"

	// synctest contexts creates a bubble of goroutines that run in a "bubble"
	// with a fake clock allowing fast execution of time-based tests.
	synctest.Run(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go measureDbDir(ctx, gaugeName, tempDir)

		for i := range 5 {
			_, err = f.Write(testData)
			require.NoError(t, err)
			// Run measureDbDir in a goroutine and stop it after a short duration
			// disk gets measured once per minute, so we have to wait for that
			time.Sleep(time.Minute + time.Millisecond)

			// Verify the gauge value matches the total size of the file
			expectedSize := int64(lenTestData * (i + 1))
			gauge := metrics.GetOrRegisterGauge(gaugeName, nil)
			require.Equal(t, expectedSize, gauge.Snapshot().Value())
		}
	})
}
