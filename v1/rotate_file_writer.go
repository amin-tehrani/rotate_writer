package rotate_writer

import (
	"errors"
	"io"
	"os"
	"path"
	"sync/atomic"
	"time"
)

type FileRotatorFn = func(RotateStatus) (rotate bool, fileName string)

type RotateFileWriter struct {
	*RotateWriter

	dir             string
	currentFilePath atomic.Value
}

func (rfw *RotateFileWriter) Open() error {

	f, err := os.OpenFile(rfw.currentFilePath.Load().(string), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	rfw.curWriter = f
	return nil
}

func (rfw *RotateFileWriter) Reset(fileName string) error {
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return err
	}

	rfw.RotateWriter.Reset(file)
	return nil
}

func (rfw *RotateFileWriter) Rotate(fileName string, newTime time.Time) error {
	file, err := os.OpenFile(path.Join(rfw.dir, fileName), os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return err
	}

	return rfw.RotateWriter.Rotate(file, newTime)
}

func NewRotateFileWriter(initFilePath string, fileRotatorFn FileRotatorFn) (*RotateFileWriter, error) {
	if fileRotatorFn == nil {
		return nil, errors.New("fileRotatorFn cannot be nil")
	}

	dir := path.Dir(initFilePath)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, err
		}
	}

	rotatorFn := func(status RotateStatus) io.WriteCloser {
		if rotate, fileName := fileRotatorFn(status); rotate {
			newFile, err := os.OpenFile(path.Join(dir, fileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				return newFile
			}
		}

		return nil
	}

	initFile, err := os.OpenFile(initFilePath, os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return nil, err
	}

	rw := NewRotateWriter(initFile, rotatorFn)
	rfw := &RotateFileWriter{
		RotateWriter: rw,
		dir:          dir,
	}

	rfw.currentFilePath.Store(initFilePath)
	return rfw, nil
}
