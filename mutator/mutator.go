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
	MutateField(dsc *desc.MessageDescriptor, msg *dynamic.Message, msgBuf *[]byte, ignoredFd []string, maxMsgSize int, rand *rand.Rand) error
	MutateMessage(dsc *desc.MessageDescriptor, msg *dynamic.Message, msgBuf *[]byte, ignoredFd []string, maxMsgSize int, rand *rand.Rand) error
}

type MultiMessageMutator interface {
	MutateField(dsc *desc.MessageDescriptor, msg *dynamic.Message, msgBuf *[]byte, valDeps []packet.MsgValDep, depMsgs []dynamic.Message, maxMsgSize int, rand *rand.Rand) error
	MutateMessage(dsc *desc.MessageDescriptor, msg *dynamic.Message, msgBuf *[]byte, valDeps []packet.MsgValDep, depMsgs []dynamic.Message, maxMsgSize int, rand *rand.Rand) error
}

type MutatorManager struct {
	smMutator     SingleMessageMutator
	mmMutator     MultiMessageMutator
	ignoredFields []string
	randSource    rand.Source
	rand          *rand.Rand
	strategy      MutationStrategy
	maxMsgSize    int
}

func (mm *MutatorManager) New(sMsgMut SingleMessageMutator, mMsgMut MultiMessageMutator, maxMsgSize int, rSrc rand.Source, ignoredFd []string, strategy MutationStrategy) {
	mm.mmMutator = mMsgMut
	mm.smMutator = sMsgMut
	mm.ignoredFields = ignoredFd
	mm.randSource = rSrc
	mm.rand = rand.New(mm.randSource)
	mm.strategy = strategy
	mm.maxMsgSize = maxMsgSize
	mm.rand.Seed(rSrc.Int63())
}

func (mm *MutatorManager) DoMutation(dsc *desc.MessageDescriptor, msg *dynamic.Message, msgBuf *[]byte) error {
	if mm.strategy == SingleField {
		return mm.smMutator.MutateField(dsc, msg, msgBuf, mm.ignoredFields, mm.maxMsgSize, mm.rand)
	} else {
		return mm.smMutator.MutateMessage(dsc, msg, msgBuf, mm.ignoredFields, mm.maxMsgSize, mm.rand)
	}
}

func (mm *MutatorManager) DoAwareMutation(dsc *desc.MessageDescriptor, msg *dynamic.Message, msgBuf *[]byte, deps []packet.MsgValDep, depMsgs []dynamic.Message) error {
	if mm.strategy == SingleField {
		return mm.mmMutator.MutateField(dsc, msg, msgBuf, deps, depMsgs, mm.maxMsgSize, mm.rand)
	} else {
		return mm.mmMutator.MutateMessage(dsc, msg, msgBuf, deps, depMsgs, mm.maxMsgSize, mm.rand)
	}
}

func (mm *MutatorManager) DoSingleMessageMutation(dsc *desc.MessageDescriptor, msg *dynamic.Message, msgBuf *[]byte) error {
	return mm.smMutator.MutateMessage(dsc, msg, msgBuf, mm.ignoredFields, mm.maxMsgSize, mm.rand)
}
