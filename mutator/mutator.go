package mutator

import (
	"math/rand"
	"time"

	"github.com/jhump/protoreflect/desc"
)

type MutationStrategy int

const (
	SingleMessage MutationStrategy = iota
	RelatedMessages
)

type SingleMessageMutator interface {
	New(string, *desc.MessageDescriptor, []string) error
	MutateField() (string, error)
	MutateMessage() (string, error)
}

type MultiMessageMutator interface {
	New(string, *desc.MessageDescriptor, []string) error
	MutateField() (string, error)
	MutateMessage() (string, error)
}

type MutatorManager struct {
	smMutator  SingleMessageMutator
	mmMutator  MultiMessageMutator
	randSource rand.Source
	rand       *rand.Rand
}

func (mm *MutatorManager) New(sMsgMut SingleMessageMutator, mMsgMut MultiMessageMutator) {
	mm.mmMutator = mMsgMut
	mm.smMutator = sMsgMut
	mm.randSource = rand.NewSource(time.Now().UnixNano())
	mm.rand = rand.New(mm.randSource)
}

func (mm *MutatorManager) DoSingleMessageMutation() (string, error) {
	return mm.smMutator.MutateMessage()
}
