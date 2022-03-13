package mutator

import (
	"math/rand"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/lukjok/gipcfuzz/packet"
	"github.com/pkg/errors"
)

type DefaultDependencyAwareMut struct {
}

func (m *DefaultDependencyAwareMut) MutateField(dsc *desc.MessageDescriptor, msg *dynamic.Message, msgBuf *[]byte, valDeps []packet.MsgValDep, depMsgs []dynamic.Message, maxMsgSize int, rand *rand.Rand) error {
	fields := dsc.GetFields()
	fieldCount := len(fields)
	mutFieldIdx := rand.Intn(fieldCount)
	msgName := dsc.GetName()
	internalIgFields := make([]string, 0, 1)

	// TODO: Need to do a refactor of this mess
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
		buf, err := msg.Marshal()
		*msgBuf = buf[:]
		if err != nil {
			return errors.WithMessage(err, "Failed to marshal the mutated message!")
		}

		return nil
	}

	if err := mutateField(fields[mutFieldIdx], msg, len(*msgBuf), maxMsgSize, rand); err != nil {
		return err
	}

	buf, err := msg.Marshal()
	*msgBuf = buf[:]
	if err != nil {
		return errors.WithMessage(err, "Failed to marshal the mutated message!")
	}

	return nil
}

func (m *DefaultDependencyAwareMut) MutateMessage(dsc *desc.MessageDescriptor, msg *dynamic.Message, msgBuf *[]byte, valDeps []packet.MsgValDep, depMsgs []dynamic.Message, maxMsgSize int, rand *rand.Rand) error {
	fields := dsc.GetFields()
	msgName := dsc.GetName()
	internalIgFields := make([]string, 0, 1)

	// TODO: Need to do a refactor of this mess
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

		if err := mutateField(field, msg, len(*msgBuf), maxMsgSize, rand); err != nil {
			return err
		}
	}

	buf, err := msg.Marshal()
	*msgBuf = buf[:]
	if err != nil {
		return errors.WithMessage(err, "Failed to marshal the mutated message!")
	}

	return nil
}
