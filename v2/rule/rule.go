package rule

import (
	"time"

	"github.com/amin-tehrani/rotate_writer/v2/metered_writer"
)

type RotateSizer interface {
	MaxSize() int
}
type RotateWriterGenerator interface {
	NewWriter(count int, createTime time.Time, prevState metered_writer.WriterState) (metered_writer.MeteredWriterCloser, error)
}
type RotateChecker interface {
	Check(state metered_writer.WriterState) bool
}

type RotateRule interface {
	RotateChecker
	RotateWriterGenerator
}
