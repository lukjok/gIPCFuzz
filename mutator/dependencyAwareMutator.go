package mutator

import (
	"math/rand"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

type DefaultDependencyAwareMut struct {
}

func (m *DefaultDependencyAwareMut) MutateField(dsc *desc.MessageDescriptor, msg *dynamic.Message, ignoredFd []string, rand *rand.Rand) (string, error) {
	return "", nil
}

func (m *DefaultDependencyAwareMut) MutateMessage(dsc *desc.MessageDescriptor, msg *dynamic.Message, ignoredFd []string, rand *rand.Rand) (string, error) {
	return "", nil
}
