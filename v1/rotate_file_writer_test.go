package rotate_writer_test

import (
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/amin-tehrani/rotate_writer/v1"
	"github.com/stretchr/testify/require"
)

func TestRotateFileWriter(t *testing.T) {
	os.RemoveAll("testdata")
	rotator := func(status rotate_writer.RotateStatus) (rotate bool, fileName string) {
		if status.CurrentSize > 0 {
			return true, fmt.Sprint("TestRotateFileWriter_", status.ItemIdx, ".txt")
		}

		return false, ""
	}

	rfw, err := rotate_writer.NewRotateFileWriter("testdata/TestRotateFileWriter_0.txt", rotator)

	require.Nil(t, err)
	require.NotNil(t, rfw)

	require.True(t, rfw.IsOpen())

	n, err := rfw.Write([]byte("1234"))

	require.Nil(t, err)
	require.Equal(t, 4, n)

	n, err = rfw.Write([]byte("5678"))

	require.Nil(t, err)
	require.Equal(t, 4, n)

	require.True(t, rfw.IsOpen())

	rfw.Close()

	require.False(t, rfw.IsOpen())

}

func TestRotateFileWriterOpenClose(t *testing.T) {
	os.RemoveAll("testdata")
	rotator := func(status rotate_writer.RotateStatus) (rotate bool, fileName string) {
		if status.CurrentSize > 10 {
			return true, fmt.Sprint("TestRotateFileWriterOpenClose_", status.ItemIdx, ".txt")
		}

		return false, ""
	}

	rfw, err := rotate_writer.NewRotateFileWriter("testdata/TestRotateFileWriterOpenClose_0.txt", rotator)

	require.Nil(t, err)
	require.NotNil(t, rfw)

	require.True(t, rfw.IsOpen())

	n, err := rfw.Write([]byte("1234"))

	require.Nil(t, err)
	require.Equal(t, 4, n)

	require.True(t, rfw.IsOpen())

	rfw.Close()

	require.False(t, rfw.IsOpen())

	n, err = rfw.Write([]byte("x"))

	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.Equal(t, 0, n)

	err = rfw.Open()

	require.Nil(t, err)
	n, err = rfw.Write([]byte("x"))

	require.Nil(t, err)
	require.Equal(t, 1, n)

	require.True(t, rfw.IsOpen())

}

func TestRotateFileWriterRotate(t *testing.T) {
	os.RemoveAll("testdata")
	rotator := func(status rotate_writer.RotateStatus) (rotate bool, fileName string) {
		if status.CurrentSize > 10 {
			return true, fmt.Sprint("TestRotateFileWriterRotate", status.ItemIdx, ".txt")
		}

		return false, ""
	}

	rfw, err := rotate_writer.NewRotateFileWriter("testdata/TestRotateFileWriterRotate_0.txt", rotator)

	require.Nil(t, err)
	require.NotNil(t, rfw)

	require.True(t, rfw.IsOpen())

	n, err := rfw.Write([]byte("1234"))

	require.Nil(t, err)
	require.Equal(t, 4, n)

	require.True(t, rfw.IsOpen())

	err = rfw.Rotate("TestRotateFileWriterRotate_1.txt", time.Now())

	require.Nil(t, err)

	require.True(t, rfw.IsOpen())

	n, err = rfw.Write([]byte("56789abc"))

	require.Nil(t, err)
	require.Equal(t, 8, n)

	rfw.Close()

	require.False(t, rfw.IsOpen())

}

func TestRotateFileWriterParallel(t *testing.T) {
	os.RemoveAll("testdata")
	rotator := func(status rotate_writer.RotateStatus) (rotate bool, fileName string) {
		if status.CurrentSize+int32(status.AddedSize) > 15 {
			return true, fmt.Sprint("TestRotateFileWriterParallel_", status.ItemIdx, "_", status.EndTime, ".txt")
		}

		return false, ""
	}

	rfw, err := rotate_writer.NewRotateFileWriter("testdata/TestRotateFileWriterParallel_0.txt", rotator)

	require.Nil(t, err)
	require.NotNil(t, rfw)

	require.True(t, rfw.IsOpen())

	wg := &sync.WaitGroup{}

	const S = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := 0; i < 5; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			s := S[i*6 : (i+1)*6]
			t.Log(s)
			n, err := rfw.Write([]byte(s))

			require.Nil(t, err)
			require.Equal(t, len(s), n)
		}(i)
	}

	wg.Wait()

	rfw.Close()
}

func TestNewRotateFileWriterNilArgs(t *testing.T) {
	_, err := rotate_writer.NewRotateFileWriter("testdata/dummy.txt", nil)
	require.Error(t, err)
	require.Equal(t, "fileRotatorFn cannot be nil", err.Error())
}

func TestRotateFileWriterReset(t *testing.T) {
	os.RemoveAll("testdata")
	rotator := func(status rotate_writer.RotateStatus) (rotate bool, fileName string) {
		return false, ""
	}

	rfw, err := rotate_writer.NewRotateFileWriter("testdata/TestReset_0.txt", rotator)
	require.Nil(t, err)

	_, err = rfw.Write([]byte("foo"))
	require.Nil(t, err)

	err = rfw.Reset("testdata/TestReset_1.txt")
	require.Nil(t, err)

	_, err = rfw.Write([]byte("bar"))
	require.Nil(t, err)

	rfw.Close()

	// Check content
	b, _ := os.ReadFile("testdata/TestReset_0.txt")
	require.Equal(t, "foo", string(b))

	b, _ = os.ReadFile("testdata/TestReset_1.txt")
	require.Equal(t, "bar", string(b))
}
