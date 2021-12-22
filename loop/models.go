package loop

import (
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/lukjok/gipcfuzz/trace"
)

type LoopMessage struct {
	Path       string
	Descriptor *desc.MessageDescriptor
	Coverage   []trace.CoverageBlock
	Energy     int
	Message    *string
}

type LoopStatus struct {
	NewPathTime      time.Time
	LastCrashTime    time.Time
	LastHangTime     time.Time
	IterationNo      int
	NewPathCount     int
	UniqueCrashCount int
	UniqueHangCount  int
	TotalExec        float64
	MsgProg          float64
}
