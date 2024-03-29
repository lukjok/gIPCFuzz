package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/lukjok/gipcfuzz/config"
	"github.com/lukjok/gipcfuzz/loop"
	"github.com/lukjok/gipcfuzz/models"
	"github.com/pterm/pterm"
	"github.com/urfave/cli/v2"
)

func main() {
	area, _ := pterm.DefaultArea.WithCenter().Start()
	ticker := time.NewTicker(1 * time.Second)
	done := make(chan bool)
	uData := make(chan *models.UIData)

	go doUIWork(*ticker, done, uData, area)

	app := &cli.App{
		Name:      "gRPCFuzz",
		Version:   "0.1",
		Compiled:  time.Now(),
		HelpName:  "contrive",
		Usage:     "a gray-box feedback-based gRPC fuzzer",
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

				ctxData := models.ContextData{
					Settings:   config,
					UIDataChan: uData,
				}

				ctx := context.WithValue(context.Background(), "data", ctxData)
				ctx, cancel := context.WithCancel(ctx)

				signalChan := make(chan os.Signal, 1)
				signal.Notify(signalChan, os.Interrupt)
				defer func() {
					signal.Stop(signalChan)
					cancel()
				}()
				go func() {
					select {
					case <-signalChan: // first signal, cancel context
						cancel()
						ticker.Stop()
						done <- true
						area.Stop()
					case <-ctx.Done():
					}
					<-signalChan // second signal, hard exit
					os.Exit(2)
				}()
				looper := loop.NewLoop(ctx)
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

func doUIWork(ticker time.Ticker, done chan bool, dChan chan *models.UIData, area *pterm.AreaPrinter) {
	sTime := time.Now()
	uiData := models.UIData{
		StartTime:     sTime,
		NewPathTime:   sTime,
		LastCrashTime: sTime,
		LastHangTime:  sTime,
		CyclesDone:    0,
		TotalPaths:    0,
		ExecSpd:       0,
		UniqCrash:     0,
		UniqHangs:     0,
		TotalExec:     0,
		CurrMsg:       "",
		MsgProg:       0,
	}

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			updateUi(area, uiData)
		case data := <-dChan:
			data.StartTime = sTime
			if data.NewPathTime.IsZero() {
				data.NewPathTime = sTime
			}
			if data.LastCrashTime.IsZero() {
				data.LastCrashTime = sTime
			}
			if data.LastHangTime.IsZero() {
				data.LastHangTime = sTime
			}
			uiData = *data
			updateUi(area, *data)
		}
	}
}

func updateUi(area *pterm.AreaPrinter, data models.UIData) {
	timing, _ := pterm.DefaultBulletList.WithItems([]pterm.BulletListItem{
		{Level: 0, Text: pterm.Gray("Run time: ") + pterm.White(fmt.Sprintf("%s ago", time.Since(data.StartTime).Round(time.Second).String()))},
		{Level: 0, Text: pterm.Gray("Last new path :") + pterm.White(fmt.Sprintf("%s ago", time.Since(data.NewPathTime).Round(time.Second).String()))},
		{Level: 0, Text: pterm.Gray("Last unique crash: ") + pterm.White(fmt.Sprintf("%s ago", time.Since(data.LastCrashTime).Round(time.Second).String()))},
		{Level: 0, Text: pterm.Gray("Last unique hang: ") + pterm.White(fmt.Sprintf("%s ago", time.Since(data.LastHangTime).Round(time.Second).String()))},
	}).Srender()
	oresults, _ := pterm.DefaultBulletList.WithItems([]pterm.BulletListItem{
		{Level: 0, Text: pterm.Gray("Current cycle: ") + pterm.White(data.CyclesDone)},
		{Level: 0, Text: pterm.Gray("Messages in queue: ") + pterm.White(data.MessageCountInQueue)},
		{Level: 0, Text: pterm.Gray("Total paths: ") + pterm.White(data.TotalPaths)},
		{Level: 0, Text: pterm.Gray("Unique crashes: ") + pterm.White(data.UniqCrash)},
		{Level: 0, Text: pterm.Gray("Unique hangs: ") + pterm.White(data.UniqHangs)},
	}).Srender()
	progress, _ := pterm.DefaultBulletList.WithItems([]pterm.BulletListItem{
		{Level: 0, Text: pterm.Gray("Total executions: ") + pterm.White(data.TotalExec)},
		{Level: 0, Text: pterm.Gray("Execution speed: ") + pterm.White(fmt.Sprintf("%.2f/s", data.ExecSpd))},
		{Level: 0, Text: pterm.Gray("Current message: ") + pterm.White(data.CurrMsg)},
		{Level: 0, Text: pterm.Gray("Message progress: ") + pterm.White(fmt.Sprintf("%.1f %%", data.MsgProg))},
	}).Srender()

	panel1 := pterm.DefaultBox.WithTitle(pterm.Green("Timing")).Sprint(timing)
	panel2 := pterm.DefaultBox.WithTitle(pterm.Green("Overall results")).Sprint(oresults)
	panel3 := pterm.DefaultBox.WithTitle(pterm.Green("Progress")).Sprint(progress)

	panels, _ := pterm.DefaultPanel.WithPanels(pterm.Panels{
		{{Data: panel1}, {Data: panel2}},
		{{Data: panel3}},
	}).Srender()

	area.Update(pterm.DefaultBox.Sprint(panels))
}
