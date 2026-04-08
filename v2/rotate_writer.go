package rotate_writer

import (
	"bytes"
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

	if rw.MeteredWriterCloser == nil {
		rw.MeteredWriterCloser = metered_writer.NewMeteredWriter(nil)
	}
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
			rw.mu.Unlock()
			return
		}
		rw.mu.Unlock()
		if err = rw.Rotate(); err != nil {
			return n, err
		}
		n2, err2 := rw.Write(p2) // Recursive
		return n + n2, err2
	}

	if rw.rule.Check(rw.State()) {
		rw.mu.Unlock()
		if err = rw.Rotate(); err != nil {
			return 0, err
		}
		return rw.Write(p) // Recursive
	}

	n, err = rw.MeteredWriterCloser.Write(p)
	rw.mu.Unlock()
	return
}

func NewRotateWriter(rule rule.RotateRule, err error) (*RotateWriter, error) {
	if err != nil {
		return nil, err
	}
	b := new([]byte)
	wc := metered_writer.NopWriterCloser(bytes.NewBuffer(*b))
	rw := RotateWriter{
		MeteredWriterCloser: metered_writer.NewMeteredWriter(wc),
		rule:                rule,
	}
	return &rw, rw.Rotate()
}
