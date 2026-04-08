package rotate_writer

import "io"

type DummyWriteCloser struct {
	writer io.Writer
}

func (dc *DummyWriteCloser) Writer() io.Writer {
	return dc.writer
}

func (dc *DummyWriteCloser) Write(p []byte) (int, error) {
	return dc.writer.Write(p)
}

func (dc *DummyWriteCloser) Close() error {
	return nil
}

func NewDummyWCloser(writer io.Writer) *DummyWriteCloser {
	return &DummyWriteCloser{
		writer: writer,
	}
}
