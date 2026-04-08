package v2

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/amin-tehrani/rotate_writer/v2/metered_writer"
	"github.com/amin-tehrani/rotate_writer/v2/rule"
)


// RotateWriter manages the writing and rotation of an io.WriteCloser based on specific conditions.
type RotateWriter struct {
	metered_writer.MeteredWriterCloser
	last metered_writer.MeteredWriterCloser

	rule rule.RotateRule

	counter atomic.Int32

	mu sync.Mutex
}

func (rw *RotateWriter) Rotate() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if err := rw.MeteredWriterCloser.Close(); err != nil {
		return err
	}
	rw.last = rw.MeteredWriterCloser

	if newWriter, err := rw.rule.NewWriter(int(rw.counter.Load()), time.Now(), rw.State()); err != nil {
		return err
	} else {
		rw.MeteredWriterCloser = newWriter
	}
	rw.counter.Add(1)

	return nil
}

func (rw *RotateWriter) Write(p []byte) (n int, err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	dataSize := len(p)
	currSize := rw.State().Size

	maxSize := -1
	if s, ok := rw.rule.(rule.RotateSizer); ok {
		maxSize = s.MaxSize()
	}

	if maxSize > 0 && int(currSize)+dataSize > maxSize {
		cap := maxSize - int(currSize)
		p1 := p[:cap]
		p2 := p[cap:]
		n, err = rw.MeteredWriterCloser.Write(p1)
		if err != nil {
			return
		}
		rw.mu.Unlock()
		if err := rw.Rotate(); err != nil {
			return 0, err
		}
		return rw.Write(p2) // Recursive
	}

	if rw.rule.Check(rw.State()) {
		rw.mu.Unlock()
		if err := rw.Rotate(); err != nil {
			return 0, err
		}
		return rw.Write(p) // Recursive
	}

	return
}

func NewRotateWriter(rule rule.RotateRule) (*RotateWriter, error) {
	rw := RotateWriter{
		rule: rule,
	}
	return &rw, rw.Rotate()
}
