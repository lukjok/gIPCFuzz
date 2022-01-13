package mutator

import (
	"encoding/hex"
	"math/rand"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/lukjok/gipcfuzz/packet"
	"github.com/pkg/errors"
)

type DefaultDependencyAwareMut struct {
}

func (m *DefaultDependencyAwareMut) MutateField(dsc *desc.MessageDescriptor, msg *dynamic.Message, valDeps []packet.MsgValDep, depMsgs []dynamic.Message, rand *rand.Rand) (string, error) {
	fields := dsc.GetFields()
	fieldCount := len(fields)
	mutFieldIdx := rand.Intn(fieldCount)
	msgName := dsc.GetName()
	internalIgFields := make([]string, 0, 1)

	for i := 0; i < len(valDeps); i++ {
		if valDeps[i].Msg1 == msgName || valDeps[i].Msg2 == msgName {
			for j := 0; j < len(fields); j++ {
				for k, v := range valDeps[i].Relations {
					if fields[j].GetFullyQualifiedName() == v || fields[j].GetFullyQualifiedName() == k {
						internalIgFields = append(internalIgFields, fields[j].GetName())
						for l := 0; l < len(depMsgs); l++ {
							if depMsgs[l].GetMessageDescriptor().GetName() == valDeps[i].Msg1 || depMsgs[l].GetMessageDescriptor().GetName() == valDeps[i].Msg2 {
								valToReplace, _ := depMsgs[l].TryGetFieldByName(fields[j].GetName())
								msg.TrySetFieldByName(fields[j].GetName(), valToReplace)
							}
						}
					}
				}
			}
		}
	}

	// Try 10 times to retry for other not ignored field
	for i := 0; i < 10; i++ {
		if isFieldIgnored(internalIgFields, fields[mutFieldIdx]) {
			mutFieldIdx = rand.Intn(fieldCount)
		} else {
			break
		}
	}

	// If no valid field was found, return not changed message
	if isFieldIgnored(internalIgFields, fields[mutFieldIdx]) {
		mMsg, err := msg.Marshal()
		if err != nil {
			return "", errors.WithMessage(err, "Failed to marshal the mutated message!")
		}

		return hex.EncodeToString(mMsg), nil
	}

	if err := mutateField(fields[mutFieldIdx], msg, rand); err != nil {
		return "", err
	}

	mMsg, err := msg.Marshal()
	if err != nil {
		return "", errors.WithMessage(err, "Failed to marshal the mutated message!")
	}

	return hex.EncodeToString(mMsg), nil
}

func (m *DefaultDependencyAwareMut) MutateMessage(dsc *desc.MessageDescriptor, msg *dynamic.Message, valDeps []packet.MsgValDep, depMsgs []dynamic.Message, rand *rand.Rand) (string, error) {
	fields := dsc.GetFields()
	msgName := dsc.GetName()
	internalIgFields := make([]string, 0, 1)

	for i := 0; i < len(valDeps); i++ {
		if valDeps[i].Msg1 == msgName || valDeps[i].Msg2 == msgName {
			for j := 0; j < len(fields); j++ {
				for k, v := range valDeps[i].Relations {
					if fields[j].GetFullyQualifiedName() == v || fields[j].GetFullyQualifiedName() == k {
						internalIgFields = append(internalIgFields, fields[j].GetName())
						for l := 0; l < len(depMsgs); l++ {
							if depMsgs[l].GetMessageDescriptor().GetName() == valDeps[i].Msg1 || depMsgs[l].GetMessageDescriptor().GetName() == valDeps[i].Msg2 {
								valToReplace, _ := depMsgs[l].TryGetFieldByName(fields[j].GetName())
								msg.TrySetFieldByName(fields[j].GetName(), valToReplace)
							}
						}
					}
				}
			}
		}
	}

	for _, field := range fields {
		if isFieldIgnored(internalIgFields, field) {
			continue
		}

		if err := mutateField(field, msg, rand); err != nil {
			return "", err
		}
	}

	mMsg, err := msg.Marshal()
	if err != nil {
		return "", errors.WithMessage(err, "Failed to marshal the mutated message!")
	}

	return hex.EncodeToString(mMsg), nil
}
