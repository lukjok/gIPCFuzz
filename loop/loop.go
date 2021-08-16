package loop

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lukjok/gipcfuzz/communication"
	"github.com/lukjok/gipcfuzz/events"
	"github.com/lukjok/gipcfuzz/memdump"
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
	MemDump        *memdump.MemoryDump
	IterationNo    int
	CurrentMessage *packet.ProtoByteMsg
}

func NewLoop(ctx context.Context) *Loop {
	ctxData := ctx.Value("data").(models.ContextData)
	if ctxData.Settings.PerformMemoryDump {
		return &Loop{
			Context: ctx,
			Output:  output.NewFilesystem(ctxData.Settings.OutputPath),
			Events:  &events.Events{},
			MemDump: memdump.NewMemoryDump(ctxData.Settings.PathToExecutable, ctxData.Settings.OutputPath, ctxData.Settings.DumpExecutablePath),
		}
	} else {
		return &Loop{
			Context: ctx,
			Output:  output.NewFilesystem(ctxData.Settings.OutputPath),
			Events:  &events.Events{},
		}
	}
}

func (l *Loop) Run() {
	l.initializeLoop()
	for idx, message := range l.Messages {
		select {
		case <-l.Context.Done():
			l.Stop()
			return
		default:
			time.Sleep(2 * time.Second)
			l.IterationNo = idx + 1
			l.CurrentMessage = &message

			resp := l.runIteration()
			if resp != nil {
				log.Printf("Got response: %s", resp)
			}
		}
	}
}

func (l *Loop) Stop() {
	log.Println("Recieved interrupt signal. Cleaning up...")
	l.Events.StopCapture()
}

func (l *Loop) runIteration() protoiface.MessageV1 {
	curIterData := l.Context.Value("data").(models.ContextData)
	endpoint := fmt.Sprintf("%s:%d", curIterData.Settings.Host, curIterData.Settings.Port)
	protoFiles := util.GetFileFullPathInDirectory(curIterData.Settings.ProtoFilesPath, []string{"Includes"})

	req := communication.GIPCRequest{
		Endpoint:          endpoint,
		Path:              l.CurrentMessage.Path,
		Data:              l.CurrentMessage.Message,
		ProtoFiles:        protoFiles,
		ProtoIncludesPath: curIterData.Settings.ProtoFilesIncludePath,
	}

	resp, err := communication.SendRequestWithMessage(req)
	if err != nil {
		l.handleIterationErr(err)
		return nil
	}
	return resp
}

func (l *Loop) runIterationWithData(path string, data *string) (protoiface.MessageV1, error) {
	curIterData := l.Context.Value("data").(models.ContextData)
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

	if len(l.Messages) == 0 {
		log.Fatal("No messages were processed! Bailing out...")
	}
	if err := l.Events.NewEventManager(events.DefaultWindowsQuery); err != nil {
		log.Printf("Error occured while creating EventManager: %s", err)
	}

	l.Events.StartCapture()
	go l.handleProcessStart()

	if loopData.Settings.DryRun {
		log.Println("Performing a dry run before starting a fuzzing process...")
		if err := l.performDryRun(); err != nil {
			log.Fatal("Dry run enabled and the process failed to respond to the valid message! Bailing out")
		}
	}
}

func (l *Loop) handleProcessStart() {
	startProgStatus := make(chan *watcher.StartProcessResponse)

	if !watcher.IsProcessRunning(l.Context) {
		go watcher.StartProcess(l.Context, startProgStatus)

		dumpPath, err := l.MemDump.StartDump()
		if err != nil {
			log.Printf("Failed to start the memory dump for the process: %s", err)
		}

		for done := false; !done; {
			select {
			case <-l.Context.Done():
				close(startProgStatus)
				done = true
			case response := <-startProgStatus:
				if response != nil && response.Error != nil {
					log.Printf("Error occurred while starting the process: %s", response.Error)
					done = true
				}
				if response != nil && response.Error == nil {
					l.writeIterationCrash(response.Output, dumpPath)
					done = true
				}
			default:
			}
		}
	}
}

func (l *Loop) writeIterationCrash(processOutput, memoryDumpPath string) {
	events := l.Events.GetEventData()
	crashOutput := output.CrashOutput{
		IterationNo:      l.IterationNo,
		MethodPath:       l.CurrentMessage.Path,
		ExecutableOutput: processOutput,
		ExecutableEvents: events,
		MemoryDumpPath:   memoryDumpPath,
	}
	if err := l.Output.SaveCrash(&crashOutput); err != nil {
		log.Printf("Failed to write iteration crash dump with error: %s", err)
	}
}

func (l *Loop) handleIterationErr(err error) {
	convertedErr := util.ConvertError(err)
	if convertedErr == models.NetworkError {
		log.Printf("gRPC request failed with error: %s", err)
		if !watcher.IsProcessRunning(l.Context) {
			go l.handleProcessStart()
		}
	}
	if convertedErr == models.UnknownError {
		log.Printf("gRPC request failed with error: %s", err)
		if !watcher.IsProcessRunning(l.Context) {
			go l.handleProcessStart()
		}
	}
}

func (l *Loop) performDryRun() error {
	sampleMessage := l.Messages[0]
	_, err := l.runIterationWithData(sampleMessage.Path, sampleMessage.Message)
	return err
}
