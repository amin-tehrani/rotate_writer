package metered_writer

import (
	"os"
)

type meteredFileWriter struct {
	baseMeteredWriter
	f *os.File
}

func (mfw *meteredFileWriter) File() *os.File {
	return mfw.f
}

func NewMeteredFileWriter(f *os.File) *meteredFileWriter {
	return &meteredFileWriter{
		f: f,
	}
}
