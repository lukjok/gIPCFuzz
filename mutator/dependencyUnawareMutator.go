package mutator

import (
	"bytes"
	"encoding/hex"
	"math"
	"math/rand"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/descriptorpb"
)

var (
	interestingFloat32 = []float32{math.MaxFloat32, math.SmallestNonzeroFloat32,
		-math.MaxFloat32, -math.SmallestNonzeroFloat32, 0.0,
		math.Nextafter32(1, 2) - 1, -(math.Nextafter32(1, 2) - 1)}
	interestingFloat64 = []float64{math.MaxFloat64, math.SmallestNonzeroFloat64,
		-math.MaxFloat64, -math.SmallestNonzeroFloat64, 0.0,
		math.Nextafter(1, 2) - 1, -(math.Nextafter(1, 2) - 1),
		math.Inf(1), math.Inf(-1), math.NaN(), -math.NaN()}
	interestingInt32  = []int32{math.MaxInt32, math.MinInt32, 0, math.MaxInt16, math.MinInt16}
	interestingInt64  = []int64{math.MaxInt64, math.MinInt64, 0, math.MaxInt32, math.MinInt32}
	interestingUint32 = []uint32{math.MaxUint32, 0, math.MaxUint16}
	interestingUint64 = []uint64{math.MaxUint64, 0, math.MaxUint32}
)

type DefaultDependencyUnawareMut struct {
}

func (m *DefaultDependencyUnawareMut) MutateField(dsc *desc.MessageDescriptor, msg *dynamic.Message, ignoredFd []string, rand *rand.Rand) (string, error) {
	// fields := dsc.GetFields()
	// mm.currentFieldIdx = 0
	// mutField := fields[mm.currentFieldIdx]

	// for i := mm.currentFieldIdx; i <= len(fields)-1; i++ {
	// 	for _, igFd := range mm.ignoredFields {
	// 		if mutField.GetName() == igFd {
	// 			mm.currentFieldIdx += 1
	// 			break
	// 		}
	// 	}
	// 	break
	// }

	// mutField = fields[mm.currentFieldIdx]
	// if err := mm.mutateField(mutField); err != nil {
	// 	return "", err
	// }

	// mMsg, err := mm.currentMessage.Marshal()
	// if err != nil {
	// 	return "", errors.WithMessage(err, "Failed to marshal the mutated message!")
	// }

	// fmt.Println(mm.currentMessage.String())
	// fmt.Println(hex.EncodeToString(mMsg))
	return "", nil
}

func (m *DefaultDependencyUnawareMut) MutateMessage(dsc *desc.MessageDescriptor, msg *dynamic.Message, ignoredFd []string, rand *rand.Rand) (string, error) {
	fields := dsc.GetFields()
	var skipField bool = false

	for _, field := range fields {
		skipField = false
		for _, igFd := range ignoredFd {
			if field.GetName() == igFd {
				skipField = true
				break
			}
		}

		if skipField {
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

func mutateField(field *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	switch field.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		if err := mutateBool(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		if err := mutateString(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		if err := mutateBytes(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		if err := mutateFloat(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		if err := mutateDouble(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_TYPE_SINT32, descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		if err := mutateInt32(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_TYPE_SINT64, descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		if err := mutateInt64(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		if err := mutateUint32(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		if err := mutateUint64(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		if err := mutateEnum(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	}
	if field.IsRepeated() {
		if err := mutateRepeated(field, msg, rand); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	}
	return nil
}

func mutateString(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	strVal := msg.GetField(fd)
	newVal := strings.Repeat(strVal.(string), rand.Intn(100))
	if len(newVal) > int(math.Pow(2, 32)) { // 2^32 is the max protobuf string length
		if err := msg.TryClearField(fd); err != nil {
			return errors.WithMessage(err, "Failed to clear string field value")
		}
		return nil
	}
	if err := msg.TrySetField(fd, newVal); err != nil {
		return errors.WithMessage(err, "Failed to change string field value")
	}
	return nil
}

func mutateBool(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	boolVal := rand.Int()%2 == 0
	if err := msg.TrySetField(fd, boolVal); err != nil {
		return errors.WithMessage(err, "Failed to change bool field value")
	}
	return nil
}

func mutateFloat(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	// float in protobuf equal to the float32 in Go
	floatVal := interestingFloat32[rand.Intn(len(interestingFloat32)-1)]
	if err := msg.TrySetField(fd, floatVal); err != nil {
		return errors.WithMessage(err, "Failed to change float field value")
	}
	return nil
}

func mutateDouble(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	// double in protobuf equal to the float64 in Go
	doubleVal := interestingFloat64[rand.Intn(len(interestingFloat64)-1)]
	if err := msg.TrySetField(fd, doubleVal); err != nil {
		return errors.WithMessage(err, "Failed to change double field value")
	}
	return nil
}

func mutateInt32(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	intVal := interestingInt32[rand.Intn(len(interestingInt32)-1)]
	if err := msg.TrySetField(fd, intVal); err != nil {
		return errors.WithMessage(err, "Failed to change Int32 field value")
	}
	return nil
}

func mutateInt64(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	intVal := interestingInt64[rand.Intn(len(interestingInt64)-1)]
	if err := msg.TrySetField(fd, intVal); err != nil {
		return errors.WithMessage(err, "Failed to change Int64 field value")
	}
	return nil
}

func mutateUint32(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	intVal := interestingUint32[rand.Intn(len(interestingUint32)-1)]
	if err := msg.TrySetField(fd, intVal); err != nil {
		return errors.WithMessage(err, "Failed to change uint32 field value")
	}
	return nil
}

func mutateUint64(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	intVal := interestingUint64[rand.Intn(len(interestingUint64)-1)]
	if err := msg.TrySetField(fd, intVal); err != nil {
		return errors.WithMessage(err, "Failed to change uint64 field value")
	}
	return nil
}

func mutateBytes(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	byteVal := msg.GetField(fd).([]byte)
	newVal := bytes.Repeat(byteVal, rand.Intn(100))
	if len(newVal) > int(math.Pow(2, 32)) { // 2^32 is the max protobuf bytes length
		if err := msg.TryClearField(fd); err != nil {
			return errors.WithMessage(err, "Failed to clear bytes field value")
		}
		return nil
	}
	if err := msg.TrySetField(fd, newVal); err != nil {
		return errors.WithMessage(err, "Failed to change bytes field value")
	}
	return nil
}

func mutateEnum(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	enum := fd.GetEnumType()
	if enum == nil {
		return errors.Errorf("Cannot get type for enum %s", fd.GetName())
	}
	enumVals := enum.GetValues()
	newEnumVal := enumVals[rand.Intn(len(enumVals)-1)].GetNumber()
	if err := msg.TrySetField(fd, newEnumVal); err != nil {
		return errors.WithMessage(err, "Failed to change enum field value")
	}
	return nil
}

func mutateRepeated(fd *desc.FieldDescriptor, msg *dynamic.Message, rand *rand.Rand) error {
	//mm.currentMessage.TrySetRepeatedField()
	// newEnumVal := enumVals[mm.rand.Intn(len(enumVals)-1)].GetNumber()
	// if err := mm.currentMessage.TrySetField(fd, newEnumVal); err != nil {
	// 	return errors.WithMessage(err, "Failed to change enum field value")
	// }
	return nil
}
