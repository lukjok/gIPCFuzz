package packet

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
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

func (h *httpStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	hstream := &httpStream{
		net:       net,
		transport: transport,
		r:         tcpreader.NewReaderStream(),
	}
	go hstream.run() // Important... we must guarantee that data from the reader stream is read.

	// ReaderStream implements tcpassembly.Stream, so we can return a pointer to it.
	return &hstream.r
}

func (h *httpStream) run() {
	buf := bufio.NewReader(&h.r)

	for {
		// const preface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
		// b := make([]byte, len(preface))
		// if _, err := io.ReadFull(buf, b); err != nil {
		// 	log.Println(err)
		// 	return
		// }
		// if string(b) != preface {
		// 	log.Println("Invalid preface")
		// 	return
		// }

		framer := http2.NewFramer(nil, buf)
		frame, err := framer.ReadFrame()
		fmt.Println(frame, err)

		if err == io.ErrUnexpectedEOF {
			return
		}

		if frame.Header().Type == http2.FrameHeaders {
			header := frame.(*http2.HeadersFrame)
			decoder := hpack.NewDecoder(4096, nil)
			hf, _ := decoder.DecodeFull(header.HeaderBlockFragment())
			for _, h := range hf {
				fmt.Printf("%s\n", h.Name+":"+h.Value)
			}
		}

		if frame.Header().Type == http2.FrameData {
			frameBuf := frame.(*http2.DataFrame).Data()
			log.Println(hex.EncodeToString(frameBuf))
		}
	}
}

func ParsePacketSource(path string) {
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
