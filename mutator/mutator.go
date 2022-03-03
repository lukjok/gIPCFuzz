package mutator

import (
	"math/rand"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/lukjok/gipcfuzz/packet"
)

type MutationStrategy int

const (
	SingleField MutationStrategy = iota
	WholeMessage
)

type SingleMessageMutator interface {
	MutateField(dsc *desc.MessageDescriptor, msg *dynamic.Message, ignoredFd []string, rand *rand.Rand) (string, error)
	MutateMessage(dsc *desc.MessageDescriptor, msg *dynamic.Message, ignoredFd []string, rand *rand.Rand) (string, error)
}

type MultiMessageMutator interface {
	MutateField(dsc *desc.MessageDescriptor, msg *dynamic.Message, valDeps []packet.MsgValDep, depMsgs []dynamic.Message, rand *rand.Rand) (string, error)
	MutateMessage(dsc *desc.MessageDescriptor, msg *dynamic.Message, valDeps []packet.MsgValDep, depMsgs []dynamic.Message, rand *rand.Rand) (string, error)
}

type MutatorManager struct {
	smMutator     SingleMessageMutator
	mmMutator     MultiMessageMutator
	ignoredFields []string
	randSource    rand.Source
	rand          *rand.Rand
	strategy      MutationStrategy
}

func (mm *MutatorManager) New(sMsgMut SingleMessageMutator, mMsgMut MultiMessageMutator, rSrc rand.Source, ignoredFd []string, strategy MutationStrategy) {
	mm.mmMutator = mMsgMut
	mm.smMutator = sMsgMut
	mm.ignoredFields = ignoredFd
	mm.randSource = rSrc
	mm.rand = rand.New(mm.randSource)
	mm.strategy = strategy
	mm.rand.Seed(rSrc.Int63())
}

func (mm *MutatorManager) DoMutation(dsc *desc.MessageDescriptor, msg *dynamic.Message) (string, error) {
	if mm.strategy == SingleField {
		return mm.smMutator.MutateField(dsc, msg, mm.ignoredFields, mm.rand)
	} else {
		return mm.smMutator.MutateMessage(dsc, msg, mm.ignoredFields, mm.rand)
	}
}

func (mm *MutatorManager) DoAwareMutation(dsc *desc.MessageDescriptor, msg *dynamic.Message, deps []packet.MsgValDep, depMsgs []dynamic.Message) (string, error) {
	if mm.strategy == SingleField {
		return mm.mmMutator.MutateField(dsc, msg, deps, depMsgs, mm.rand)
	} else {
		return mm.mmMutator.MutateMessage(dsc, msg, deps, depMsgs, mm.rand)
	}
}

func (mm *MutatorManager) DoSingleMessageMutation(dsc *desc.MessageDescriptor, msg *dynamic.Message) (string, error) {
	return mm.smMutator.MutateMessage(dsc, msg, mm.ignoredFields, mm.rand)
}
