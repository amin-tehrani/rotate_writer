package metered_writer

import (
	"io"
	"sync"
	"time"
)

type MeteredWriterCloser interface {
	io.WriteCloser
	State() WriterState
}
type baseMeteredWriter struct {
	io.WriteCloser
	state WriterState
	mu    sync.RWMutex
}

func (m *baseMeteredWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	n, err = m.WriteCloser.Write(p)
	if err != nil {
		return
	}
	m.state.Size += int64(n)
	m.state.Modified = time.Now()
	return
}

func (m *baseMeteredWriter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Closed != nil {
		return io.ErrClosedPipe
	}
	err := m.WriteCloser.Close()
	now := time.Now()
	m.state.Closed = &now
	return err
}

func (m *baseMeteredWriter) State() WriterState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}
