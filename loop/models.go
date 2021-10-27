package loop

import (
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
