package mutator

import "github.com/jhump/protoreflect/desc"

type Mutator interface {
	New(string, *desc.MessageDescriptor) error
	MutateField() (string, error)
	MutateMessage() (string, error)
}

// type MutatedMessage struct {
// 	originalMessage *dynamic.Message
// 	currentMessage  *dynamic.Message
// }
