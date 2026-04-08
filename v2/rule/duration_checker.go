package rule

import (
	"time"

	"github.com/amin-tehrani/rotate_writer/v2/metered_writer"
)

type durationRotateChecker struct {
	duration time.Duration
}

func (drc *durationRotateChecker) Check(state metered_writer.WriterState) bool {
	return state.Modified.Sub(state.Created) > drc.duration
}
