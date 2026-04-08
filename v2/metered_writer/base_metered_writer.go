package metered_writer

import (
	"io"
	"os"
	"sync"
	"time"
)

type MeteredWriterCloser interface {
	io.WriteCloser
	State() WriterState
}

type nopWriterCloser struct {
	io.Writer
}

func (*nopWriterCloser) Close() error {
	return nil
}

func NopWriterCloser(w io.Writer) io.WriteCloser {
	return &nopWriterCloser{Writer: w}
}

type baseMeteredWriter struct {
	w     io.WriteCloser
	state WriterState
	mu    sync.RWMutex
}

func (m *baseMeteredWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	n, err = m.w.Write(p)
	if err != nil {
		return
	}
	m.state.Size += int64(n)
	m.state.ModifiedAt = time.Now()
	return
}

func (m *baseMeteredWriter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.ClosedAt != nil {
		return io.ErrClosedPipe
	}
	err := m.w.Close()
	now := time.Now()
	m.state.ClosedAt = &now
	return err
}

func (m *baseMeteredWriter) State() WriterState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func NewMeteredWriter(w io.WriteCloser) *baseMeteredWriter {
	if w == nil {
		devNull, _ := os.Open(os.DevNull)
		w = devNull
	}
	return &baseMeteredWriter{
		w: w,
		state: WriterState{
			CreatedAt: time.Now(),
		},
	}
}
