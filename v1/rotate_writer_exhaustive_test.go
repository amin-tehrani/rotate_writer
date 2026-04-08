package rotate_writer_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/amin-tehrani/rotate_writer/v1"
	"github.com/stretchr/testify/require"
)

// High concurrency test for NewRotateWriter
func TestRotateWriterHighConcurrency(t *testing.T) {
	writers := []*rotate_writer.DummyWriteCloser{
		newDummyBuffer(),
	}
	var mu sync.Mutex

	rotatorFn := func(s rotate_writer.RotateStatus) io.WriteCloser {
		if s.CurrentSize > 100 {
			newWriter := newDummyBuffer()
			mu.Lock()
			writers = append(writers, newWriter)
			mu.Unlock()
			return newWriter
		}
		return nil
	}

	rw := rotate_writer.NewRotateWriter(writers[0], rotatorFn)

	var wg sync.WaitGroup
	var totalBytes atomic.Int32

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				n, err := rw.Write([]byte(fmt.Sprintf("%04d", idx)))
				require.NoError(t, err)
				totalBytes.Add(int32(n))
			}
		}(i)
	}

	wg.Wait()
	err := rw.Close()
	require.NoError(t, err)

	require.Equal(t, int32(100*50*4), totalBytes.Load())

	// Verify the written total bytes across all buffers matches expectations
	var writtenTotal int
	mu.Lock()
	defer mu.Unlock()
	for _, w := range writers {
		bufStr := w.Writer().(*bytes.Buffer).String()
		writtenTotal += len(bufStr)
	}
	require.Equal(t, int(totalBytes.Load()), writtenTotal)
}

// High concurrency race testing for RotateFileWriter
func TestRotateFileWriterRace(t *testing.T) {
	dir := "testdata/race"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)

	rotator := func(status rotate_writer.RotateStatus) (rotate bool, fileName string) {
		if status.CurrentSize > 200 {
			return true, fmt.Sprintf("file_%d.txt", status.ItemIdx)
		}
		return false, ""
	}

	rfw, err := rotate_writer.NewRotateFileWriter(path.Join(dir, "file_0.txt"), rotator)
	require.NoError(t, err)

	var wg sync.WaitGroup
	var totalWritten atomic.Int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				n, err := rfw.Write([]byte("xxxxxx"))
				require.NoError(t, err)
				totalWritten.Add(int32(n))
			}
		}(i)
	}

	wg.Wait()
	err = rfw.Close()
	require.NoError(t, err)

	require.Equal(t, int32(50*20*6), totalWritten.Load())

	files, err := os.ReadDir(dir)
	require.NoError(t, err)

	var allSize int64
	for _, f := range files {
		info, err := f.Info()
		require.NoError(t, err)
		allSize += info.Size()
	}
	require.Equal(t, int64(totalWritten.Load()), allSize)
}

// Test accurate rotation boundary
func TestRotateWriterExactBoundary(t *testing.T) {
	writers := []*rotate_writer.DummyWriteCloser{
		newDummyBuffer(),
	}

	rotatorFn := func(s rotate_writer.RotateStatus) io.WriteCloser {
		if s.CurrentSize >= 10 {
			newWriter := newDummyBuffer()
			writers = append(writers, newWriter)
			return newWriter
		}
		return nil
	}

	rw := rotate_writer.NewRotateWriter(writers[0], rotatorFn)

	// Write 10 bytes across two calls - should exactly hit 10 and next one rotates
	_, err := rw.Write([]byte("12345"))
	require.NoError(t, err)

	require.Len(t, writers, 1)

	_, err = rw.Write([]byte("67890")) // Now size is 10
	require.NoError(t, err)

	require.Len(t, writers, 1)

	// The next write should cause a rotation
	_, err = rw.Write([]byte("A"))
	require.NoError(t, err)

	require.Len(t, writers, 2)
}