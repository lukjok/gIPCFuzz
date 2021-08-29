package packet

import (
	"encoding/hex"
	"fmt"

	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
)

func CalculateReqResRelations(msgs []ProtoByteMsg) {
	aMsgs := make([]ProtoByteMsg, 0, 10)
	for _, msg := range msgs {
		if msg.Type == Request || msg.Type == Response {
			aMsgs = append(aMsgs, msg)
		}
	}

	deps := make(map[string]string)

	k := 10 // Window size

	// Message m[kIdx2] will apprear after m[kIdx1]
	for i := 0; i < k-1; i++ {
		if aMsgs[i].Type != Request || aMsgs[i+1].Type != Response {
			continue
		}
		kIdx1 := m[requestMsgs[i].Path]
		kIdx2 := m[requestMsgs[i+1].Path]

	}

	lWMsg := requestMsgs[k-1]

	for i := 1; i < len(requestMsgs)-k; i++ {
		kIdx1 := m[lWMsg.Path]
		kIdx2 := m[requestMsgs[i+k-1].Path]
		matrix[kIdx1][kIdx2] += 1
		lWMsg = requestMsgs[i+k-1]
	}
}

func DissectMsgsCommonFields(msg1, msg2 ProtoByteMsg) ([]MsgValDep, error) {
	relations := make([]MsgValDep, 0, 2)
	if len(*msg1.Message) == 0 || len(*msg2.Message) == 0 {
		return nil, errors.New("Message contents should not be empty!")
	}
	buf1, err := hex.DecodeString(*msg1.Message)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to decode msg1 message")
	}
	buf2, err := hex.DecodeString(*msg2.Message)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to decode msg2 message")
	}

	pbMsg1 := dynamic.NewMessage(msg1.Descriptor)
	pbMsg2 := dynamic.NewMessage(msg2.Descriptor)
	if err := pbMsg1.Unmarshal(buf1); err != nil {
		return nil, errors.WithMessage(err, "Failed to unmarshal msg1 contents!")
	}
	if err := pbMsg2.Unmarshal(buf2); err != nil {
		return nil, errors.WithMessage(err, "Failed to unmarshal msg2 contents!")
	}

	mFd1 := msg1.Descriptor.GetFields()
	mFd2 := msg2.Descriptor.GetFields()
	for _, fd1 := range mFd1 {
		deps := make(map[string]string)
		for _, fd2 := range mFd2 {
			if fd1.GetType() != fd2.GetType() {
				continue
			}
			val1 := pbMsg1.GetField(fd1)
			val2 := pbMsg2.GetField(fd2)
			if val1 == val2 {
				deps[fd1.GetName()] = fd2.GetName()
			}
		}
		relations = append(relations, MsgValDep{
			Msg1: msg1.Descriptor.GetName(),
			Msg2: msg2.Descriptor.GetName(),
		})
	}
	return relations, nil
}

func CalculateRelationMatrix(msgs []ProtoByteMsg) {
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
	fmt.Println(k)
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
