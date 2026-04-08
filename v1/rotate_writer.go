package rotate_writer

import (
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// RotateStatus holds the rotation-related metadata including the index of the item,
// the current size, the added size, and the start/end times of the current writing period.
type RotateStatus struct {
	ItemIdx     int32
	CurrentSize int32
	AddedSize   int
	StartTime   time.Time
	EndTime     time.Time
}

// RotatorFn defines a function type that takes a RotateStatus and returns an io.WriteCloser.
// This function determines when and how to rotate the writer.
// if the function returns nil, the writer will not be rotated.
type RotatorFn = func(RotateStatus) io.WriteCloser

// RotateWriter manages the writing and rotation of an io.WriteCloser based on specific conditions.
type RotateWriter struct {
	curWriter io.WriteCloser

	curSize      atomic.Int32
	curStartTime atomic.Int64

	counter atomic.Int32

	rotatorFn RotatorFn

	mu sync.Mutex
}

// Status returns the current rotation status of the writer.
func (rw *RotateWriter) Status() RotateStatus {
	return RotateStatus{
		ItemIdx:     rw.counter.Load(),
		CurrentSize: rw.curSize.Load(),
		AddedSize:   0,
		StartTime:   time.Unix(rw.curStartTime.Load(), 0),
		EndTime:     time.Now(),
	}
}

func (rw *RotateWriter) checkRotate(len_p int, newTime time.Time) io.WriteCloser {
	startTimeInt := rw.curStartTime.Load()
	startTime := time.Unix(startTimeInt, 0)

	status := RotateStatus{
		ItemIdx:     rw.counter.Load() + 1,
		CurrentSize: rw.curSize.Load(),
		AddedSize:   len_p,
		StartTime:   startTime,
		EndTime:     newTime,
	}
	return rw.rotatorFn(status)
}

func (rw *RotateWriter) Rotate(newWriter io.WriteCloser, newTime time.Time) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	return rw.rotate(newWriter, newTime)
}

// Rotate rotates the writer with the given io.WriteCloser and time.Time.
// Returns an error if the writer could not be rotated.
func (rw *RotateWriter) rotate(newWriter io.WriteCloser, newTime time.Time) error {
	if rw.curWriter != nil {
		err := rw.curWriter.Close()
		if err != nil {
			return err
		}
	}
	rw.curWriter = newWriter

	rw.curStartTime.Store(newTime.Unix())
	rw.counter.Add(1)
	rw.curSize.Store(0)

	return nil
}

// Write writes the given bytes to the current writer.
// If a new writer is needed, it will be rotated.
// Returns io.ErrClosedPipe if the writer is closed.
func (rw *RotateWriter) Write(p []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	newTime := time.Now()
	newWriter := rw.checkRotate(len(p), newTime)

	if newWriter != nil {
		err := rw.rotate(newWriter, newTime)
		if err != nil {
			return 0, &RotateError{err}
		}
	}

	if rw.curWriter == nil {
		return 0, io.ErrClosedPipe
	}

	n, err := rw.curWriter.Write(p)

	if err != nil {
		return 0, err
	}

	rw.curSize.Add(int32(n))

	return n, nil
}

// Reset resets the writer to the initial state.
func (rw *RotateWriter) Reset(writerCloser io.WriteCloser) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.curWriter = writerCloser
	rw.curStartTime.Store(time.Now().Unix())
	rw.counter.Store(0)
	rw.curSize.Store(0)
}

// IsOpen returns true if the writer is open.
func (rfw *RotateFileWriter) IsOpen() bool {
	return rfw.curWriter != nil
}

// Close closes the current writer.
func (rw *RotateWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.curWriter == nil {
		return nil
	}
	err := rw.curWriter.Close()
	rw.curWriter = nil
	return err
}

// NewRotateWriter creates a new RotateWriter.
// If the initialWriter or rotateCondition is nil, returns nil.
func NewRotateWriter(initialWriter io.WriteCloser, rotateCondition RotatorFn) *RotateWriter {
	if rotateCondition == nil || initialWriter == nil {
		return nil
	}

	rw := &RotateWriter{
		rotatorFn: rotateCondition,
	}

	rw.Reset(initialWriter)

	return rw
}
