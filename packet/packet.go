//Part of the code used from the https://gist.github.com/siddontang/b23b891a5afa9ea88b63932626a2a486

package packet

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/lukjok/gipcfuzz/util"
	"github.com/pkg/errors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// httpStreamFactory implements tcpassembly.StreamFactory
type httpStreamFactory struct{}

// httpStream will handle the actual decoding of http requests.
type httpStream struct {
	net, transport gopacket.Flow
	r              tcpreader.ReaderStream
}

var pathMsgsCount int32 = 0
var pathMsgs []ProtoByteMsg = make([]ProtoByteMsg, 0, 10)
var streamPath = map[string]map[uint32]string{}
var protoDescriptors []*desc.FileDescriptor
var pathLock sync.RWMutex

func (h *httpStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	hstream := &httpStream{
		net:       net,
		transport: transport,
		r:         tcpreader.NewReaderStream(),
	}
	go hstream.run()
	return &hstream.r
}

func GetParsedMessages(path string, protoPath string, protoIncludePath []string) []ProtoByteMsg {
	LoadProtoDescriptions(
		util.GetFileNamesInDirectory(protoPath, []string{"Includes"}),
		append(protoIncludePath, protoPath))
	ProcessPacketSource(path)
	return pathMsgs
}

func LoadProtoDescriptions(files []string, includePaths []string) {
	parser := protoparse.Parser{}
	var err error
	parser.ImportPaths = includePaths
	protoDescriptors, err = parser.ParseFiles(files...)
	if err != nil {
		log.Fatalln("Failed to read proto descriptions", err)
	}
}

func ProcessPacketSource(path string) {
	handle, err := pcap.OpenOffline(path)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	packets := gopacket.NewPacketSource(
		handle, handle.LinkType()).Packets()

	streamFactory := &httpStreamFactory{}
	streamPool := tcpassembly.NewStreamPool(streamFactory)
	assembler := tcpassembly.NewAssembler(streamPool)
	ticker := time.Tick(time.Minute)

	for {
		select {
		case packet := <-packets:
			// A nil packet indicates the end of a pcap file.
			if packet == nil {
				return
			}

			if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
				//log.Println("Unusable packet")
				continue
			}
			tcp := packet.TransportLayer().(*layers.TCP)
			assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), tcp, packet.Metadata().Timestamp)

		case <-ticker:
			// Every minute, flush connections that haven't seen activity in the past 2 minutes.
			assembler.FlushOlderThan(time.Now().Add(time.Minute * -2))
		}
	}
}

func (h *httpStream) run() {
	buf := bufio.NewReader(&h.r)
	framer := http2.NewFramer(ioutil.Discard, buf)
	framer.MaxHeaderListSize = uint32(16 << 20)
	framer.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	net := fmt.Sprintf("%s:%s -> %s:%s", h.net.Src(), h.transport.Src(), h.net.Dst(), h.transport.Dst())
	revNet := fmt.Sprintf("%s:%s -> %s:%s", h.net.Dst(), h.transport.Dst(), h.net.Src(), h.transport.Src())
	// 1 request, 2 response, 0 unkonwn
	var streamSide = map[uint32]int{}

	defer func() {
		pathLock.Lock()
		delete(streamPath, net)
		delete(streamPath, revNet)
		pathLock.Unlock()
	}()

	for {
		peekBuf, err := buf.Peek(9)
		if err == io.EOF {
			return
		} else if err != nil {
			log.Print("Error reading frame", h.net, h.transport, ":", err)
			continue
		}

		prefix := string(peekBuf)

		if strings.HasPrefix(prefix, "PRI") {
			buf.Discard(len(http2.ClientPreface))
		}

		frame, err := framer.ReadFrame()
		if err == io.EOF {
			return
		}

		if err != nil {
			log.Print("Error reading frame", h.net, h.transport, ":", err)
			continue
		}

		id := frame.Header().StreamID
		switch frame := frame.(type) {
		case *http2.MetaHeadersFrame:
			for _, hf := range frame.Fields {
				if hf.Name == ":path" {
					// TODO: remove stale stream ID
					pathLock.Lock()
					_, ok := streamPath[net]
					if !ok {
						streamPath[net] = map[uint32]string{}
					}
					streamPath[net][id] = hf.Value
					pathLock.Unlock()
					streamSide[id] = 1
				} else if hf.Name == ":status" {
					streamSide[id] = 2
				}
			}
		case *http2.DataFrame:
			var path string
			pathLock.RLock()
			nets, ok := streamPath[net]
			if !ok {
				nets, ok = streamPath[revNet]
			}

			if ok {
				path = nets[id]
			}

			pathLock.RUnlock()
			if msg, err := ParseFrameToByteMsg(net, path, frame, streamSide[id]); err == nil {
				pathMsgs = append(pathMsgs, msg)
				pathMsgsCount++
			}
		default:
		}
	}
}

func ParseFrameToByteMsg(net string, path string, frame *http2.DataFrame, side int) (ProtoByteMsg, error) {
	buf := frame.Data()
	id := frame.Header().StreamID
	compress := buf[0]

	if len(buf) == 0 {
		return ProtoByteMsg{
			Path:       path,
			Type:       MessageType(side),
			StreamID:   0,
			Descriptor: nil,
			Message:    nil,
		}, errors.New("Message length is zero!")
	}

	// if side != 1 {
	// 	return ProtoByteMsg{
	// 		Path:       path,
	// 		Type:       MessageType(side),
	// 		Descriptor: nil,
	// 		Message:    nil,
	// 	}, &proto.ParseError{}
	// }

	if compress == 1 {
		// use compression, check Message-Encoding later
		log.Printf("%s %d use compression, msg %q", net, id, buf[5:])
		return ProtoByteMsg{
			Path:       path,
			Type:       MessageType(side),
			StreamID:   0,
			Descriptor: nil,
			Message:    nil,
		}, errors.New("Message is using compression!")
	}

	if len(protoDescriptors) > 0 {
		for _, dscr := range protoDescriptors {
			oldPath := strings.Replace(path[1:], "/", ".", 1)
			sym := dscr.FindSymbol(oldPath)
			if sym != nil {
				mDsc := sym.(*desc.MethodDescriptor)
				encMsg := hex.EncodeToString(buf[5:])
				if MessageType(side) == Request {
					return ProtoByteMsg{
						Path:       path[1:],
						Type:       MessageType(side),
						StreamID:   id,
						Descriptor: mDsc.GetInputType(),
						Message:    &encMsg,
					}, nil
				} else {
					return ProtoByteMsg{
						Path:       path[1:],
						Type:       MessageType(side),
						StreamID:   id,
						Descriptor: mDsc.GetOutputType(),
						Message:    &encMsg,
					}, nil
				}

			} else {
				continue
			}
		}
	}

	return ProtoByteMsg{
		Path:       path,
		Type:       MessageType(side),
		Descriptor: nil,
		StreamID:   0,
		Message:    nil,
	}, errors.New("No proto descriptors were found!")
}

func ParseFrame(net string, path string, frame *http2.DataFrame, side int) (ProtoMsg, error) {
	buf := frame.Data()
	id := frame.Header().StreamID
	compress := buf[0]

	if side != 1 {
		return ProtoMsg{
			Path:    path,
			Type:    MessageType(side),
			Message: nil,
		}, &proto.ParseError{}
	}

	if compress == 1 {
		// use compression, check Message-Encoding later
		log.Printf("%s %d use compression, msg %q", net, id, buf[5:])
		return ProtoMsg{
			Path:    path,
			Type:    MessageType(side),
			Message: nil,
		}, &proto.ParseError{}
	}

	if len(protoDescriptors) > 0 {
		for _, dscr := range protoDescriptors {
			msg := dscr.AsProto()
			oldPath := strings.Replace(path[1:], "/", ".", 1)
			sym := dscr.FindSymbol(oldPath)
			if sym != nil {
				if err := proto.Unmarshal(buf[5:], msg); err == nil {
					log.Printf("%s %d %s %s", net, id, path, msg)
					return ProtoMsg{
						Path:    path[1:],
						Type:    MessageType(side),
						Message: msg,
					}, nil
				} else {
					log.Println(err)
				}
			} else {
				continue
			}
		}
	}

	return ProtoMsg{
		Path:    path,
		Type:    MessageType(side),
		Message: nil,
	}, &proto.ParseError{}
}

func dumpProto(net string, id uint32, path string, buf []byte) {
	var out bytes.Buffer
	if err := decodeProto(&out, buf, 0); err != nil {
		// decode failed
		log.Printf("%s %d %s %q", net, id, path, buf)
	} else {
		log.Printf("%s %d %s\n%s", net, id, path, out.String())

	}
}

func decodeProto(out *bytes.Buffer, buf []byte, depth int) error {
out:
	for {
		if len(buf) == 0 {
			return nil
		}

		for i := 0; i < depth; i++ {
			out.WriteString("  ")
		}

		op, n := proto.DecodeVarint(buf)
		if n == 0 {
			return io.ErrUnexpectedEOF
		}

		buf = buf[n:]

		tag := op >> 3
		wire := op & 7

		switch wire {
		default:
			fmt.Fprintf(out, "tag=%d unknown wire=%d\n", tag, wire)
			break out
		case proto.WireBytes:
			l, n := proto.DecodeVarint(buf)
			if n == 0 {
				return io.ErrUnexpectedEOF
			}
			buf = buf[n:]
			if len(buf) < int(l) {
				return io.ErrUnexpectedEOF
			}

			// Here we can't know the raw bytes is string, or embedded message
			// So we try to parse like a embedded message at first
			outLen := out.Len()
			fmt.Fprintf(out, "tag=%d struct\n", tag)
			if err := decodeProto(out, buf[0:int(l)], depth+1); err != nil {
				// Seem this is not a embedded message, print raw buffer
				out.Truncate(outLen)
				fmt.Fprintf(out, "tag=%d bytes=%q\n", tag, buf[0:int(l)])
			}
			buf = buf[l:]
		case proto.WireFixed32:
			if len(buf) < 4 {
				return io.ErrUnexpectedEOF
			}
			u := binary.LittleEndian.Uint32(buf[0:4])
			buf = buf[4:]
			fmt.Fprintf(out, "tag=%d fix32=%d\n", tag, u)
		case proto.WireFixed64:
			if len(buf) < 8 {
				return io.ErrUnexpectedEOF
			}
			u := binary.LittleEndian.Uint64(buf[0:8])
			buf = buf[4:]
			fmt.Fprintf(out, "tag=%d fix64=%d\n", tag, u)
		case proto.WireVarint:
			u, n := proto.DecodeVarint(buf)
			if n == 0 {
				return io.ErrUnexpectedEOF
			}
			buf = buf[n:]
			fmt.Fprintf(out, "tag=%d varint=%d\n", tag, u)
		case proto.WireStartGroup:
			fmt.Fprintf(out, "tag=%d start\n", tag)
			depth++
		case proto.WireEndGroup:
			fmt.Fprintf(out, "tag=%d end\n", tag)
			depth--
		}
	}
	return io.ErrUnexpectedEOF
}
