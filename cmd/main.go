package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lukjok/gipcfuzz/communication"
	"github.com/lukjok/gipcfuzz/config"
	"github.com/lukjok/gipcfuzz/packet"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:      "gRPCFuzz",
		Version:   "0.1",
		Compiled:  time.Now(),
		HelpName:  "contrive",
		Usage:     "a grey-box feedback-based gRPC fuzzer",
		UsageText: "gipcfuzz [options] [arguments]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "cfg",
				Aliases: []string{"c"},
				Value:   "..\\config.json",
				Usage:   "Path to the configuration file",
			},
		},
		Action: func(c *cli.Context) error {
			msgs := packet.GetParsedMessages("C:\\Users\\lukas\\Documents\\gIRPCStuff\\tp_grpc_traffic.pcapng",
				"C:\\Users\\lukas\\Documents\\gIRPCStuff\\HelloProtos",
				"C:\\Users\\lukas\\Documents\\gIRPCStuff\\HelloProtos\\Includes")
			log.Print(msgs)
			cfgPath := c.String("cfg")
			if len(cfgPath) == 0 {
				config := config.ParseConfigurationFile(cfgPath)
				log.Println("Starting gIPCFuzz...")

				endpoint := fmt.Sprintf("%s:%d", config.Host, config.Port)
				data := "0a064a6572656d79"
				protoFiles := []string{"C:\\Users\\lukas\\Downloads\\grpc-go-course-master\\hello\\helloclient\\hellopb\\hello.proto"}
				importPath := []string{"C:\\Users\\lukas\\Downloads\\grpc-go-course-master\\hello\\helloclient\\hellopb"}
				req := communication.GIPCRequest{
					Endpoint:          endpoint,
					Path:              "hello.helloService/Hello",
					Data:              &data,
					ProtoPath:         protoFiles,
					ProtoIncludesPath: importPath,
				}
				ret := communication.SendRequest(req)
				if ret {
					log.Println("Sent the request!")
				}

			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
