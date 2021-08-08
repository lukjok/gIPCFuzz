package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/lukjok/gipcfuzz/config"
	"github.com/lukjok/gipcfuzz/events"
	"github.com/lukjok/gipcfuzz/loop"
	"github.com/lukjok/gipcfuzz/models"
	"github.com/lukjok/gipcfuzz/output"
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
			cfgPath := c.String("cfg")
			if len(cfgPath) != 0 {
				config := config.ParseConfigurationFile(cfgPath)

				log.Println("Starting gIPCFuzz...")
				ctxData := models.ContextData{
					Settings: config,
				}
				ctx := context.WithValue(context.Background(), "data", ctxData)
				looper := loop.Loop{
					Context: ctx,
					Output:  output.NewFilesystem("C:\\gIPCFuzz\\Output"),
					Events:  &events.Events{},
				}
				looper.Run()

			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
