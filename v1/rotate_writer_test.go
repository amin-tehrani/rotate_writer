package rotate_writer_test

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/amin-tehrani/rotate_writer/v1"
	"github.com/stretchr/testify/require"
)

func newDummyBuffer() *rotate_writer.DummyWriteCloser {
	return rotate_writer.NewDummyWCloser(bytes.NewBuffer(nil))
}

func TestRotateWriterDummyBuffer(t *testing.T) {

	writers := []*rotate_writer.DummyWriteCloser{
		newDummyBuffer(),
	}

	rotatorFn := func(s rotate_writer.RotateStatus) io.WriteCloser {
		if s.CurrentSize >= 4 {
			newWriter := newDummyBuffer()
			writers = append(writers, newWriter)
			return newWriter
		} else {
			return nil
		}

	}

	rw := rotate_writer.NewRotateWriter(writers[0], rotatorFn)

	n, err := rw.Write([]byte("1234"))

	require.Nil(t, err)
	require.Equal(t, 4, n)

	require.Len(t, writers, 1)

	n, err = rw.Write([]byte("56789"))

	require.Nil(t, err)
	require.Equal(t, 5, n)

	require.Len(t, writers, 2)

	require.Equal(t, "1234", string(writers[0].Writer().(*bytes.Buffer).Bytes()))
	require.Equal(t, "56789", string(writers[1].Writer().(*bytes.Buffer).Bytes()))

	currStatus := rw.Status()
	require.Equal(t, int32(1), currStatus.ItemIdx)
	require.Equal(t, int32(5), currStatus.CurrentSize)
	require.Equal(t, 0, currStatus.AddedSize)
}

func TestRotateWriterDummyBufferParallel(t *testing.T) {

	writers := []*rotate_writer.DummyWriteCloser{
		newDummyBuffer(),
	}
	var mu sync.Mutex

	rotatorFn := func(s rotate_writer.RotateStatus) io.WriteCloser {
		if s.CurrentSize > 0 {
			newWriter := newDummyBuffer()
			mu.Lock()
			writers = append(writers, newWriter)
			mu.Unlock()
			return newWriter
		} else {
			return nil
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(3)

	rw := rotate_writer.NewRotateWriter(writers[0], rotatorFn)

	go func() {
		defer wg.Done()
		n, err := rw.Write([]byte("1234"))
		require.Nil(t, err)
		require.Equal(t, 4, n)
	}()

	go func() {
		defer wg.Done()
		n, err := rw.Write([]byte("56789"))
		require.Nil(t, err)
		require.Equal(t, 5, n)
	}()

	go func() {
		defer wg.Done()
		n, err := rw.Write([]byte("abc"))
		require.Nil(t, err)
		require.Equal(t, 3, n)
	}()

	wg.Wait()

}

func TestNewRotateWriterNilArgs(t *testing.T) {
	rw := rotate_writer.NewRotateWriter(nil, nil)
	require.Nil(t, rw)

	rw = rotate_writer.NewRotateWriter(newDummyBuffer(), nil)
	require.Nil(t, rw)
}

type errorCloseWriter struct {
	io.Writer
}

func (e *errorCloseWriter) Close() error {
	return io.ErrClosedPipe
}

func TestRotateError(t *testing.T) {
	writers := []io.WriteCloser{&errorCloseWriter{bytes.NewBuffer(nil)}}
	rotatorFn := func(s rotate_writer.RotateStatus) io.WriteCloser {
		if s.CurrentSize > 0 {
			return newDummyBuffer()
		}
		return nil
	}
	rw := rotate_writer.NewRotateWriter(writers[0], rotatorFn)
	
	// This write will succeed.
	rw.Write([]byte("foo"))
	
	// This write will trigger rotation, and the closing of errorCloseWriter will fail.
	_, err := rw.Write([]byte("bar"))
	require.Error(t, err)

	_, ok := err.(*rotate_writer.RotateError)
	require.True(t, ok)
	require.Equal(t, "failed to rotate file: io: read/write on closed pipe", err.Error())
}

type errorWriteWriter struct {
	io.WriteCloser
}

func (e *errorWriteWriter) Write(p []byte) (n int, err error) {
	return 0, io.ErrShortWrite
}

func TestWriteError(t *testing.T) {
	rw := rotate_writer.NewRotateWriter(&errorWriteWriter{newDummyBuffer()}, func(rotate_writer.RotateStatus) io.WriteCloser { return nil })
	_, err := rw.Write([]byte("foo"))
	require.Equal(t, io.ErrShortWrite, err)
}
