package mutator

import (
	"math/rand"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

type MutationStrategy int

const (
	SingleMessage MutationStrategy = iota
	RelatedMessages
)

type SingleMessageMutator interface {
	MutateField(dsc *desc.MessageDescriptor, msg *dynamic.Message, ignoredFd []string, rand *rand.Rand) (string, error)
	MutateMessage(dsc *desc.MessageDescriptor, msg *dynamic.Message, ignoredFd []string, rand *rand.Rand) (string, error)
}

type MultiMessageMutator interface {
	MutateField(dsc *desc.MessageDescriptor, msg *dynamic.Message, ignoredFd []string, rand *rand.Rand) (string, error)
	MutateMessage(dsc *desc.MessageDescriptor, msg *dynamic.Message, ignoredFd []string, rand *rand.Rand) (string, error)
}

type MutatorManager struct {
	smMutator     SingleMessageMutator
	mmMutator     MultiMessageMutator
	ignoredFields []string
	randSource    rand.Source
	rand          *rand.Rand
}

func (mm *MutatorManager) New(sMsgMut SingleMessageMutator, mMsgMut MultiMessageMutator, rSrc rand.Source, ignoredFd []string) {
	mm.mmMutator = mMsgMut
	mm.smMutator = sMsgMut
	mm.ignoredFields = ignoredFd
	mm.randSource = rSrc
	mm.rand = rand.New(mm.randSource)
}

func (mm *MutatorManager) DoSingleMessageMutation(dsc *desc.MessageDescriptor, msg *dynamic.Message) (string, error) {
	return mm.smMutator.MutateMessage(dsc, msg, mm.ignoredFields, mm.rand)
}
