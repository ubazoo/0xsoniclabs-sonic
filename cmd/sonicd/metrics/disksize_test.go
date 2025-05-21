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
	f := createTestFile(t, tempDir, "file1")

	gaugeName := "testGauge"

	// synctest contexts creates a bubble of goroutines that run in a "bubble"
	// with a fake clock allowing fast execution of time-based tests.
	synctest.Run(func() {

		ctx, cancel := context.WithCancel(t.Context())
		go measureDbDir(ctx, gaugeName, tempDir)

		for i := range 5 {
			writeTestData(t, f, testData)

			// Run measureDbDir in a goroutine and stop it after a short duration
			// disk gets measured once per minute, so we have to wait for that
			time.Sleep(time.Minute + time.Millisecond)

			// Verify the gauge value matches the total size of the file
			expectedSize := int64(lenTestData * (i + 1))
			snapshotValueEquals(t, gaugeName, expectedSize)
		}
		cancel()
	})
}

func TestMeasureDbDir_LoopCanBeCancelled(t *testing.T) {
	tempDir := t.TempDir()
	done := make(chan struct{})

	ctx, cancel := context.WithCancel(t.Context())
	// create a channel to signal when the loop has stopped
	go func() {
		// channel will only be closed when the loop stops and measureDbDir returns
		defer close(done)
		measureDbDir(ctx, "testGauge", tempDir)
	}()
	// enforce cancel right after the goroutine starts
	cancel()

	select {
	case <-done: // Success: the goroutine exited
	case <-time.After(1 * time.Second):
		t.Fatal("timeout: measureDbDir did not stop after cancellation")
	}
}

func TestSetDataDir_TracksExpectedDir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create some data to write to the test file
	testData := []byte("test data")
	lenTestData := len(testData)

	// Create a test file in the temporary directory
	f := createTestFile(t, tempDir, "file1")
	defer f.Close()
	writeTestData(t, f, testData)

	f2 := createTestFile(t, tempDir, "carmen")
	defer f2.Close()
	writeTestData(t, f2, testData)

	gaugeName := "db_size"
	carmenGaugeName := "statedb/disksize"

	// synctest contexts creates a bubble of goroutines that run in a "bubble"
	// with a fake clock allowing fast execution of time-based tests.
	synctest.Run(func() {
		ctx, cancel := context.WithCancel(t.Context())
		SetDataDir(ctx, tempDir)

		time.Sleep(time.Minute + time.Millisecond)
		cancel()

		// Verify the gauge value matches the total size of the file
		expectedSize := int64(lenTestData)
		// db_size counts the size of all files in the directory
		snapshotValueEquals(t, gaugeName, expectedSize*2)

		// statedb/disksize counts the size of the carmen file
		snapshotValueEquals(t, carmenGaugeName, expectedSize)
	})
}

func TestSetDataDir_StopsTrackingWhenCancelled(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create some data to write to the test file
	testData := []byte("test data")

	// Create a test file in the temporary directory
	f := createTestFile(t, tempDir, "file1")
	defer f.Close()

	gaugeName := "db_size"

	synctest.Run(func() {
		ctx, cancel := context.WithCancel(t.Context())
		SetDataDir(ctx, tempDir)
		cancel()

		time.Sleep(time.Minute + time.Millisecond)
		writeTestData(t, f, testData)
		// Since data was written after the cancel, the gauge should not
		// reflect the new size
		snapshotValueEquals(t, gaugeName, int64(0))
	})
}

// createTestFile is a helper functions that creates a file. It should be closed
// in a defer after the call to this function.
func createTestFile(t *testing.T, dir string, name string) *os.File {
	testFile := filepath.Join(dir, name)
	f, err := os.OpenFile(testFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	require.NoError(t, err)
	return f
}

// writeTestData is a helper function that writes data to a file.
func writeTestData(t *testing.T, f *os.File, data []byte) {
	_, err := f.Write(data)
	require.NoError(t, err)
}

// snapshotValueEquals is a helper function that gets the gauge with the given
// name and checks if the gauge value equals the expected value.
func snapshotValueEquals(t *testing.T, gaugeName string, expectedSize int64) {
	gauge := metrics.GetOrRegisterGauge(gaugeName, nil)
	require.Equal(t, expectedSize, gauge.Snapshot().Value())
}
