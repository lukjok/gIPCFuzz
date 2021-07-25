package communication

import (
	"context"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

const statusCodeOffset = 64

var (
	exit                    = os.Exit
	connectTimeout *float64 = new(float64)
	keepaliveTime  *float64 = new(float64)
	maxMsgSz       *int     = new(int)
	plaintext      *bool    = new(bool)
	insecure       *bool    = new(bool)
	cacert         *string  = new(string)
	cert           *string  = new(string)
	key            *string  = new(string)
	serverName     *string  = new(string)
	authority      *string  = new(string)
	isUnixSocket   func() bool
	target         string
	symbol         string
	addlHeaders    []string
	rpcHeaders     []string
	reflHeaders    []string
)

func SendRequest(request GIPCRequest) bool {
	isPlainHelper := bool(true)
	plaintext = &isPlainHelper

	emptyStringHelper := string("")
	cacert = &emptyStringHelper
	cert = &emptyStringHelper
	key = &emptyStringHelper
	serverName = &emptyStringHelper
	authority = &emptyStringHelper

	target = request.Endpoint
	symbol = request.Path

	var cc *grpc.ClientConn
	var descSource DescriptorSource
	var refClient *grpcreflect.Client
	var fileSource DescriptorSource

	ctx := context.Background()

	dial := func() *grpc.ClientConn {
		dialTime := 10 * time.Second
		if *connectTimeout > 0 {
			dialTime = time.Duration(*connectTimeout * float64(time.Second))
		}
		ctx, cancel := context.WithTimeout(ctx, dialTime)
		defer cancel()
		var opts []grpc.DialOption
		if *keepaliveTime > 0 {
			timeout := time.Duration(*keepaliveTime * float64(time.Second))
			opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:    timeout,
				Timeout: timeout,
			}))
		}
		if *maxMsgSz > 0 {
			opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(*maxMsgSz)))
		}
		var creds credentials.TransportCredentials
		if !*plaintext {
			var err error
			creds, err = ClientTransportCredentials(*insecure, *cacert, *cert, *key)
			if err != nil {
				log.Fatal(err, "Failed to configure transport credentials")
			}

			// can use either -servername or -authority; but not both
			if *serverName != "" && *authority != "" {
				if *serverName == *authority {
					log.Print("Both -servername and -authority are present; prefer only -authority.")
				} else {
					log.Fatal(nil, "Cannot specify different values for -servername and -authority.")
				}
			}
			overrideName := *serverName
			if overrideName == "" {
				overrideName = *authority
			}

			if overrideName != "" {
				if err := creds.OverrideServerName(overrideName); err != nil {
					log.Fatal(err, "Failed to override server name as %q", overrideName)
				}
			}
		} else if *authority != "" {
			opts = append(opts, grpc.WithAuthority(*authority))
		}

		grpcurlUA := "gipcfuzz"
		opts = append(opts, grpc.WithUserAgent(grpcurlUA))

		network := "tcp"
		if isUnixSocket != nil && isUnixSocket() {
			network = "unix"
		}
		cc, err := BlockingDial(ctx, network, target, creds, opts...)
		if err != nil {
			log.Fatal(err, "Failed to dial target host %q", target)
		}
		return cc
	}

	if len(request.ProtoPath) > 0 {
		var err error
		fileSource, err = DescriptorSourceFromProtoFiles(request.ProtoIncludesPath, request.ProtoPath...)
		if err != nil {
			log.Fatal(err, "Failed to process proto source files.")
		}
	}

	descSource = fileSource

	reset := func() {
		if refClient != nil {
			refClient.Reset()
			refClient = nil
		}
		if cc != nil {
			cc.Close()
			cc = nil
		}
	}
	defer reset()
	exit = func(code int) {
		// since defers aren't run by os.Exit...
		reset()
		os.Exit(code)
	}

	if cc == nil {
		cc = dial()
	}
	var in io.Reader = strings.NewReader(*request.Data)

	options := FormatOptions{
		EmitJSONDefaultFields: false,
		IncludeTextSeparator:  true,
		AllowUnknownFields:    false,
	}
	rf, formatter, err := RequestParserAndFormatter(FormatRaw, descSource, in, options)
	if err != nil {
		log.Fatal(err, "Failed to construct request parser and formatter for %q", "JSON")
	}
	h := &DefaultEventHandler{
		Out:            os.Stdout,
		Formatter:      formatter,
		VerbosityLevel: 0,
	}
	err = InvokeRPC(ctx, descSource, cc, symbol, append(addlHeaders, rpcHeaders...), h, rf.Next)
	if err != nil {
		if errStatus, ok := status.FromError(err); ok {
			h.Status = errStatus
		} else {
			log.Fatal(err, "Error invoking method %q", symbol)
		}
	}

	if h.Status.Code() != codes.OK {
		//printFormattedStatus(os.Stderr, h.Status, formatter)
		PrintStatus(os.Stderr, h.Status, formatter)
		exit(statusCodeOffset + int(h.Status.Code()))
	}

	return true
}
