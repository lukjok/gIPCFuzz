package packet

import (
	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
)

type MessageType int

const (
	Unknown MessageType = iota
	Request
	Response
)

type ProtoMsg struct {
	Path    string
	Type    MessageType
	Message proto.Message
}

type ProtoByteMsg struct {
	Path       string
	Type       MessageType
	Descriptor *desc.MessageDescriptor
	Message    *string
}
