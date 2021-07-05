package main

import (
	"fmt"
	"log"
	"os"
	"time"

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
				Name:    "arg",
				Aliases: []string{"a"},
				Value:   "Test",
				Usage:   "Sample argument",
			},
		},
		Action: func(c *cli.Context) error {
			fmt.Printf("%#v\n", c.String("arg"))
			if c.Bool("infinite") {
				c.App.Run([]string{})
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
