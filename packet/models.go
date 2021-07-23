package packet

import "github.com/golang/protobuf/proto"

type MessageType int

const (
	Request MessageType = iota
	Response
	Unknown
)

type ProtoMsg struct {
	Path    string
	Type    MessageType
	Message proto.Message
}
