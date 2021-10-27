package packet

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/descriptorpb"
)

func CalculateReqResRelations(msgs []ProtoByteMsg) []MsgValDep {
	aMsgs := sortReqResOrder(msgs)
	rels := make([]MsgValDep, 0, 5)
	keys := make(map[string]bool)

	// Message m[kIdx2] will apprear after m[kIdx1]
	// Ensure to always start from request
	for i := 0; i < len(aMsgs)-2; i++ {
		dp, err := DissectMsgsCommonFields(aMsgs[i+1], aMsgs[i+2])
		if err != nil {
			continue
		}
		mKey := fmt.Sprintf("%s:%s", dp.Msg1, dp.Msg2)
		if _, value := keys[mKey]; !value {
			rels = append(rels, dp)
			keys[mKey] = true
		}
	}

	return rels
}

//Orders message list in this order [rqMsg1; rsMsg1; rqMsgn; rsMsgn]
func sortReqResOrder(msgs []ProtoByteMsg) []ProtoByteMsg {
	sortedMsgs := make([]ProtoByteMsg, 0, 1)
	keys := make(map[int]bool)
	for i := 0; i < len(msgs); i++ {
		if _, value := keys[int(msgs[i].StreamID)]; !value && msgs[i].Type == Request {
			for j := 0; j < len(msgs); j++ {
				if msgs[i].StreamID == msgs[j].StreamID && msgs[j].Type == Response {
					sortedMsgs = append(sortedMsgs, msgs[i])
					sortedMsgs = append(sortedMsgs, msgs[j])
					keys[int(msgs[i].StreamID)] = true
					break
				}
			}
		}
	}
	return sortedMsgs
}

func DissectMsgsCommonFields(msg1, msg2 ProtoByteMsg) (MsgValDep, error) {
	if msg1.Path == msg2.Path {
		return MsgValDep{}, errors.New("Both messages are for the same method!")
	}
	if len(*msg1.Message) == 0 || len(*msg2.Message) == 0 {
		return MsgValDep{}, errors.New("Message contents should not be empty!")
	}
	buf1, err := hex.DecodeString(*msg1.Message)
	if err != nil {
		return MsgValDep{}, errors.WithMessage(err, "Failed to decode msg1 message")
	}
	buf2, err := hex.DecodeString(*msg2.Message)
	if err != nil {
		return MsgValDep{}, errors.WithMessage(err, "Failed to decode msg2 message")
	}

	pbMsg1 := dynamic.NewMessage(msg1.Descriptor)
	pbMsg2 := dynamic.NewMessage(msg2.Descriptor)
	if err := pbMsg1.Unmarshal(buf1); err != nil {
		return MsgValDep{}, errors.WithMessage(err, "Failed to unmarshal msg1 contents!")
	}
	if err := pbMsg2.Unmarshal(buf2); err != nil {
		return MsgValDep{}, errors.WithMessage(err, "Failed to unmarshal msg2 contents!")
	}

	deps := make(map[string]string)
	mFd1 := msg1.Descriptor.GetFields()
	mFd2 := msg2.Descriptor.GetFields()
	for _, fd1 := range mFd1 {
		for _, fd2 := range mFd2 {

			if fd1.IsRepeated() && !fd2.IsRepeated() {
				getRelationshipsFromRepMessages(fd1, fd2, pbMsg1, pbMsg2, deps)
			}

			if fd2.IsRepeated() && !fd1.IsRepeated() {
				getRelationshipsFromRepMessages(fd2, fd1, pbMsg2, pbMsg1, deps)
			}

			if fd1.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && !fd1.IsRepeated() {
				getRelationshipsFromMessages(fd1, fd2, pbMsg1, pbMsg2, deps)
			}

			if fd2.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && !fd2.IsRepeated() {
				getRelationshipsFromMessages(fd2, fd1, pbMsg2, pbMsg1, deps)
			}

			getRelationshipsFromFields(fd1, fd2, pbMsg1, pbMsg2, deps)
		}
	}

	if len(deps) > 0 {
		return MsgValDep{
			Msg1:      msg1.Descriptor.GetName(),
			Msg2:      msg2.Descriptor.GetName(),
			Relations: deps,
		}, nil
	} else {
		return MsgValDep{
			Msg1:      msg1.Descriptor.GetName(),
			Msg2:      msg2.Descriptor.GetName(),
			Relations: deps,
		}, errors.New("No dependencies were found!")
	}
}

func getRelationshipsFromFields(fd1, fd2 *desc.FieldDescriptor, msg1, msg2 *dynamic.Message, deps map[string]string) {
	val1 := msg1.GetField(fd1)
	val2 := msg2.GetField(fd2)
	fName1 := strings.ToLower(fd1.GetName())
	fName2 := strings.ToLower(fd2.GetName())

	if fd1.GetType() != fd2.GetType() {
		return
	}

	if fName1 == fName2 {
		if val1 == val2 {
			deps[fd1.GetFullyQualifiedName()] = fd2.GetFullyQualifiedName()
		}
	}

	if strings.Contains(fName1, fName2) || strings.Contains(fName2, fName1) {
		if val1 == val2 {
			deps[fd1.GetFullyQualifiedName()] = fd2.GetFullyQualifiedName()
		}
	}
}

func getRelationshipsFromRepMessages(fd1, fd2 *desc.FieldDescriptor, msg1, msg2 *dynamic.Message, deps map[string]string) {
	val1 := msg1.GetField(fd1)
	val2 := msg2.GetField(fd2)
	fName := strings.ToLower(fd2.GetName())

	if valIntArr, ok := val1.([]interface{}); ok {
		for _, intArrVal := range valIntArr {
			if arrMsg, ok := intArrVal.(*dynamic.Message); ok {
				rDescs := arrMsg.GetKnownFields()
				for _, fdR := range rDescs {
					fRName := strings.ToLower(fdR.GetName())
					valR := arrMsg.GetField(fdR)

					//if types are not identical, skip this iteration
					if fdR.GetType() != fd2.GetType() {
						continue
					}

					if fRName == fName {
						//if its ENUM and both fields are same enum, make relationship
						if fdR.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM && fdR.GetEnumType() == fd2.GetEnumType() {
							deps[fd2.GetFullyQualifiedName()] = fdR.GetFullyQualifiedName()
						} else {
							if valR == val2 {
								deps[fd2.GetFullyQualifiedName()] = fdR.GetFullyQualifiedName()
							}
						}

						// if field name contains substring of other field name, additionally check the value equality
					} else if strings.Contains(fRName, fName) || strings.Contains(fName, fRName) {
						//if its ENUM and both fields are same enum, make relationship
						if fdR.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM && fdR.GetEnumType() == fd2.GetEnumType() {
							deps[fd2.GetFullyQualifiedName()] = fdR.GetFullyQualifiedName()
						} else {
							if valR == val2 {
								deps[fd2.GetFullyQualifiedName()] = fdR.GetFullyQualifiedName()
							}
						}
					}
				}
			}
		}
	}
}

func getRelationshipsFromMessages(fd1, fd2 *desc.FieldDescriptor, msg1, msg2 *dynamic.Message, deps map[string]string) {
	val1 := msg1.GetField(fd1)
	val2 := msg2.GetField(fd2)
	fName := strings.ToLower(fd2.GetName())

	if valMsg, ok := val1.(*dynamic.Message); ok {
		mDescs := valMsg.GetKnownFields()
		for _, fdM := range mDescs {
			fMName := strings.ToLower(fdM.GetName())
			valM := valMsg.GetField(fdM)

			//if types are not identical, skip this iteration
			if fdM.GetType() != fd2.GetType() {
				continue
			}

			if fMName == fName {
				//if its ENUM and both fields are same enum, make relationship
				if fdM.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM && fdM.GetEnumType() == fd2.GetEnumType() {
					deps[fd2.GetFullyQualifiedName()] = fdM.GetFullyQualifiedName()
				} else {
					deps[fd2.GetFullyQualifiedName()] = fdM.GetFullyQualifiedName()
				}

				// if field name contains substring of other field name, additionally check the value equality
			} else if strings.Contains(fMName, fName) || strings.Contains(fName, fMName) {
				//if its ENUM and both fields are same enum, make relationship
				if fdM.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM && fdM.GetEnumType() == fd2.GetEnumType() {
					deps[fd2.GetFullyQualifiedName()] = fdM.GetFullyQualifiedName()
				}
				if valM == val2 {
					deps[fd2.GetFullyQualifiedName()] = fdM.GetFullyQualifiedName()
				}
			}
		}
	}
}

func CalculateRelationMatrix(msgs []ProtoByteMsg) ([][]float32, map[string]int) {
	requestMsgs := make([]ProtoByteMsg, 0, 10)
	for _, msg := range msgs {
		if msg.Type == Request {
			requestMsgs = append(requestMsgs, msg)
		}
	}

	uniqMsgs := unique(requestMsgs)

	matrix := make([][]float32, len(uniqMsgs))
	for idx := range matrix {
		matrix[idx] = make([]float32, len(uniqMsgs))
	}

	m := make(map[string]int)
	for i := 0; i < len(uniqMsgs); i++ {
		m[uniqMsgs[i].Path] = i
	}

	k := 10 // Window size

	// Message m[kIdx2] will apprear after m[kIdx1]
	for i := 0; i < k-1; i++ {
		kIdx1 := m[requestMsgs[i].Path]
		kIdx2 := m[requestMsgs[i+1].Path]
		matrix[kIdx1][kIdx2] += 1
	}

	lWMsg := requestMsgs[k-1]

	for i := 1; i < len(requestMsgs)-k; i++ {
		kIdx1 := m[lWMsg.Path]
		kIdx2 := m[requestMsgs[i+k-1].Path]
		matrix[kIdx1][kIdx2] += 1
		lWMsg = requestMsgs[i+k-1]
	}

	normalize(matrix, len(uniqMsgs))
	return matrix, m
}

func normalize(m [][]float32, size int) {
	for i := 0; i < size; i++ {
		var sum float32 = 0
		for j := 0; j < size; j++ {
			sum += m[i][j]
		}
		for j := 0; j < size; j++ {
			m[i][j] /= sum
		}
	}
}

func unique(msgs []ProtoByteMsg) []ProtoByteMsg {
	keys := make(map[string]bool)
	list := []ProtoByteMsg{}
	for _, entry := range msgs {
		if _, value := keys[entry.Path]; !value {
			keys[entry.Path] = true
			list = append(list, entry)
		}
	}
	return list
}
