package loop

import (
	"context"
	"fmt"
	"log"

	"github.com/lukjok/gipcfuzz/communication"
	"github.com/lukjok/gipcfuzz/events"
	"github.com/lukjok/gipcfuzz/models"
	"github.com/lukjok/gipcfuzz/output"
	"github.com/lukjok/gipcfuzz/packet"
	"github.com/lukjok/gipcfuzz/util"
	"github.com/lukjok/gipcfuzz/watcher"
	"google.golang.org/protobuf/runtime/protoiface"
)

type LoopManager interface {
	Run()
	Stop()
}

type Loop struct {
	Context        context.Context
	Messages       []packet.ProtoByteMsg
	Output         *output.Filesystem
	Events         *events.Events
	IterationNo    int
	CurrentMessage *packet.ProtoByteMsg
}

func (l *Loop) Run() {
	l.initializeLoop()
	for idx, message := range l.Messages {
		l.IterationNo = idx + 1
		l.CurrentMessage = &message
		resp, err := runIteration(l.Context, message.Path, message.Message)
		l.handleIterationErr(l.Context, err)
		if err == nil {
			log.Printf("Got response: %s", resp)
		}
	}
}

func (l *Loop) Stop() {

}

func runIteration(ctx context.Context, path string, data *string) (protoiface.MessageV1, error) {
	curIterData := ctx.Value("data").(models.ContextData)

	endpoint := fmt.Sprintf("%s:%d", curIterData.Settings.Host, curIterData.Settings.Port)
	protoFiles := util.GetFileFullPathInDirectory(curIterData.Settings.ProtoFilesPath, []string{"Includes"})
	req := communication.GIPCRequest{
		Endpoint:          endpoint,
		Path:              path,
		Data:              data,
		ProtoFiles:        protoFiles,
		ProtoIncludesPath: curIterData.Settings.ProtoFilesIncludePath,
	}

	return communication.SendRequestWithMessage(req)
}

func (l *Loop) initializeLoop() {
	loopData := l.Context.Value("data").(models.ContextData)
	l.Messages = packet.GetParsedMessages(
		loopData.Settings.PcapFilePath,
		loopData.Settings.ProtoFilesPath,
		loopData.Settings.ProtoFilesIncludePath)
	if err := l.Events.NewEventManager(events.DefaultWindowsQuery); err != nil {
		log.Printf("Error occured while creating EventManager: %s", err)
	}
	l.Events.StartCapture()
	go l.handleProcessStart(l.Context)
}

func (l *Loop) handleProcessStart(ctx context.Context) {
	startProgStatus := make(chan *watcher.StartProcessResponse)
	if !watcher.IsProcessRunning(ctx) {
		go watcher.StartProcess(ctx, startProgStatus)
		for done := false; !done; {
			select {
			case response := <-startProgStatus:
				if response != nil && response.Error != nil {
					log.Printf("Error occurred while starting the process: %s", response.Error)
					done = true
				}
				if response != nil && response.Error == nil {
					fmt.Print(response.Output)
					l.writeIterationCrash(response.Output)
					done = true
				}
			default:
			}
		}
	}
}

func (l *Loop) writeIterationCrash(out string) {
	events := l.Events.GetEventData()
	crashOutput := output.CrashOutput{
		IterationNo:      l.IterationNo,
		MethodPath:       l.CurrentMessage.Path,
		ExecutableOutput: out,
		ExecutableEvents: events,
		MemoryDumpPath:   "",
	}
	if err := l.Output.SaveCrash(&crashOutput); err != nil {
		log.Printf("Failed to write iteration crash dump with error: %s", err)
	}
}

func (l *Loop) handleIterationErr(ctx context.Context, err error) {
	convertedErr := util.ConvertError(err)
	if convertedErr == models.NetworkError {
		log.Printf("gRPC request failed with error: %s", err)
		if !watcher.IsProcessRunning(ctx) {
			go l.handleProcessStart(ctx)
		}
	}
	if convertedErr == models.UnknownError {
		log.Printf("gRPC request failed with error: %s", err)
		if !watcher.IsProcessRunning(ctx) {
			go l.handleProcessStart(ctx)
		}
	}
}
