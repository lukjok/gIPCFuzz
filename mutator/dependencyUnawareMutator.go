package mutator

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

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

type MutatedMessage struct {
	originalMessage *dynamic.Message
	currentMessage  *dynamic.Message
	descriptor      *desc.MessageDescriptor
	ignoredFields   []string
	currentFieldIdx int
	randSource      rand.Source
	rand            *rand.Rand
}

func (mm *MutatedMessage) New(message string, dsc *desc.MessageDescriptor, ignoredFd []string) error {
	if len(message) == 0 {
		return errors.New("Message contents should not be empty!")
	}
	buf, err := hex.DecodeString(message)
	if err != nil {
		return errors.WithMessage(err, "Failed to initialize mutated message")
	}
	mm.currentMessage = dynamic.NewMessage(dsc)
	if err := mm.currentMessage.Unmarshal(buf); err != nil {
		return errors.WithMessage(err, "Failed to unmarshal message contents!")
	}

	mm.ignoredFields = ignoredFd
	mm.descriptor = dsc
	mm.randSource = rand.NewSource(time.Now().UnixNano())
	mm.rand = rand.New(mm.randSource)
	return nil
}

func (mm *MutatedMessage) MutateField() (string, error) {
	fields := mm.descriptor.GetFields()
	mm.currentFieldIdx = 0
	mutField := fields[mm.currentFieldIdx]

	for i := mm.currentFieldIdx; i <= len(fields)-1; i++ {
		for _, igFd := range mm.ignoredFields {
			if mutField.GetName() == igFd {
				mm.currentFieldIdx += 1
				break
			}
		}
		break
	}

	mutField = fields[mm.currentFieldIdx]
	if err := mm.mutateField(mutField); err != nil {
		return "", err
	}

	mMsg, err := mm.currentMessage.Marshal()
	if err != nil {
		return "", errors.WithMessage(err, "Failed to marshal the mutated message!")
	}

	fmt.Println(mm.currentMessage.String())
	fmt.Println(hex.EncodeToString(mMsg))
	return hex.EncodeToString(mMsg), nil
}

func (mm *MutatedMessage) MutateMessage() (string, error) {
	fmt.Println(mm.currentMessage.String())
	fields := mm.descriptor.GetFields()
	var skipField bool = false

	for _, field := range fields {
		skipField = false
		for _, igFd := range mm.ignoredFields {
			if field.GetName() == igFd {
				skipField = true
				break
			}
		}

		if skipField {
			continue
		}

		if err := mm.mutateField(field); err != nil {
			return "", err
		}
	}

	mMsg, err := mm.currentMessage.Marshal()
	if err != nil {
		return "", errors.WithMessage(err, "Failed to marshal the mutated message!")
	}

	fmt.Println(mm.currentMessage.String())
	fmt.Println(hex.EncodeToString(mMsg))
	return hex.EncodeToString(mMsg), nil
}

func (mm *MutatedMessage) mutateField(field *desc.FieldDescriptor) error {
	switch field.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		if err := mm.MutateBool(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		if err := mm.MutateString(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		if err := mm.MutateBytes(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		if err := mm.MutateFloat(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		if err := mm.MutateDouble(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_TYPE_SINT32, descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		if err := mm.MutateInt32(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_TYPE_SINT64, descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		if err := mm.MutateInt64(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		if err := mm.MutateUint32(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		if err := mm.MutateUint64(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		if err := mm.MutateEnum(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	}
	if field.IsRepeated() {
		if err := mm.MutateRepeated(field); err != nil {
			return errors.WithMessage(err, "MutateMessage failed")
		}
	}
	return nil
}

func (mm *MutatedMessage) MutateString(fd *desc.FieldDescriptor) error {
	strVal := mm.currentMessage.GetField(fd)
	newVal := strings.Repeat(strVal.(string), mm.rand.Intn(100))
	if len(newVal) > int(math.Pow(2, 32)) { // 2^32 is the max protobuf string length
		if err := mm.currentMessage.TryClearField(fd); err != nil {
			return errors.WithMessage(err, "Failed to clear string field value")
		}
		return nil
	}
	if err := mm.currentMessage.TrySetField(fd, newVal); err != nil {
		return errors.WithMessage(err, "Failed to change string field value")
	}
	return nil
}

func (mm *MutatedMessage) MutateBool(fd *desc.FieldDescriptor) error {
	boolVal := mm.rand.Int()%2 == 0
	if err := mm.currentMessage.TrySetField(fd, boolVal); err != nil {
		return errors.WithMessage(err, "Failed to change bool field value")
	}
	return nil
}

func (mm *MutatedMessage) MutateFloat(fd *desc.FieldDescriptor) error {
	// float in protobuf equal to the float32 in Go
	floatVal := interestingFloat32[mm.rand.Intn(len(interestingFloat32)-1)]
	if err := mm.currentMessage.TrySetField(fd, floatVal); err != nil {
		return errors.WithMessage(err, "Failed to change float field value")
	}
	return nil
}

func (mm *MutatedMessage) MutateDouble(fd *desc.FieldDescriptor) error {
	// double in protobuf equal to the float64 in Go
	doubleVal := interestingFloat64[mm.rand.Intn(len(interestingFloat64)-1)]
	if err := mm.currentMessage.TrySetField(fd, doubleVal); err != nil {
		return errors.WithMessage(err, "Failed to change double field value")
	}
	return nil
}

func (mm *MutatedMessage) MutateInt32(fd *desc.FieldDescriptor) error {
	intVal := interestingInt32[mm.rand.Intn(len(interestingInt32)-1)]
	if err := mm.currentMessage.TrySetField(fd, intVal); err != nil {
		return errors.WithMessage(err, "Failed to change Int32 field value")
	}
	return nil
}

func (mm *MutatedMessage) MutateInt64(fd *desc.FieldDescriptor) error {
	intVal := interestingInt64[mm.rand.Intn(len(interestingInt64)-1)]
	if err := mm.currentMessage.TrySetField(fd, intVal); err != nil {
		return errors.WithMessage(err, "Failed to change Int64 field value")
	}
	return nil
}

func (mm *MutatedMessage) MutateUint32(fd *desc.FieldDescriptor) error {
	intVal := interestingUint32[mm.rand.Intn(len(interestingUint32)-1)]
	if err := mm.currentMessage.TrySetField(fd, intVal); err != nil {
		return errors.WithMessage(err, "Failed to change uint32 field value")
	}
	return nil
}

func (mm *MutatedMessage) MutateUint64(fd *desc.FieldDescriptor) error {
	intVal := interestingUint64[mm.rand.Intn(len(interestingUint64)-1)]
	if err := mm.currentMessage.TrySetField(fd, intVal); err != nil {
		return errors.WithMessage(err, "Failed to change uint64 field value")
	}
	return nil
}

func (mm *MutatedMessage) MutateBytes(fd *desc.FieldDescriptor) error {
	byteVal := mm.currentMessage.GetField(fd).([]byte)
	newVal := bytes.Repeat(byteVal, mm.rand.Intn(100))
	if len(newVal) > int(math.Pow(2, 32)) { // 2^32 is the max protobuf bytes length
		if err := mm.currentMessage.TryClearField(fd); err != nil {
			return errors.WithMessage(err, "Failed to clear bytes field value")
		}
		return nil
	}
	if err := mm.currentMessage.TrySetField(fd, newVal); err != nil {
		return errors.WithMessage(err, "Failed to change bytes field value")
	}
	return nil
}

func (mm *MutatedMessage) MutateEnum(fd *desc.FieldDescriptor) error {
	enum := fd.GetEnumType()
	if enum == nil {
		return errors.Errorf("Cannot get type for enum %s", fd.GetName())
	}
	enumVals := enum.GetValues()
	newEnumVal := enumVals[mm.rand.Intn(len(enumVals)-1)].GetNumber()
	if err := mm.currentMessage.TrySetField(fd, newEnumVal); err != nil {
		return errors.WithMessage(err, "Failed to change enum field value")
	}
	return nil
}

func (mm *MutatedMessage) MutateRepeated(fd *desc.FieldDescriptor) error {
	//mm.currentMessage.TrySetRepeatedField()
	// newEnumVal := enumVals[mm.rand.Intn(len(enumVals)-1)].GetNumber()
	// if err := mm.currentMessage.TrySetField(fd, newEnumVal); err != nil {
	// 	return errors.WithMessage(err, "Failed to change enum field value")
	// }
	return nil
}
