package metered_writer

import (
	"time"
)

type WriterState struct {
	Created  time.Time
	Modified time.Time
	Closed   *time.Time
	Size     int64
}
