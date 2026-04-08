package metered_writer

import (
	"fmt"
	"time"
)

type WriterState struct {
	CreatedAt  time.Time
	ModifiedAt time.Time
	ClosedAt   *time.Time
	Size       int64
}

func (ws WriterState) String() string {
	return fmt.Sprintf("%#v", ws)
}
