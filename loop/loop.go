package loop

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"time"

	"github.com/lukjok/gipcfuzz/communication"
	"github.com/lukjok/gipcfuzz/config"
	"github.com/lukjok/gipcfuzz/events"
	"github.com/lukjok/gipcfuzz/memdump"
	"github.com/lukjok/gipcfuzz/models"
	"github.com/lukjok/gipcfuzz/mutator"
	"github.com/lukjok/gipcfuzz/output"
	"github.com/lukjok/gipcfuzz/packet"
	"github.com/lukjok/gipcfuzz/trace"
	"github.com/lukjok/gipcfuzz/util"
	"github.com/lukjok/gipcfuzz/watcher"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/runtime/protoiface"
)

type LoopManager interface {
	Run()
	Stop()
}

type Loop struct {
	Context        context.Context
	Messages       []LoopMessage
	ValueDeps      []packet.MsgValDep
	MsgDeps        [][]float32
	MsgDepMap      map[string]int
	Output         *output.Filesystem
	Events         *events.Events
	MemDump        *memdump.MemoryDump
	Trace          *trace.Trace
	IterationNo    int
	CurrentMessage *LoopMessage
}

func NewLoop(ctx context.Context) *Loop {
	ctxData := ctx.Value("data").(models.ContextData)
	tm, err := trace.NewTraceManager()
	if err != nil {
		log.Fatalf("Failed to initialize tracing manager!")
	}
	if ctxData.Settings.PerformMemoryDump {
		return &Loop{
			Context: ctx,
			Output:  output.NewFilesystem(ctxData.Settings.OutputPath),
			Events:  &events.Events{},
			Trace:   tm,
			MemDump: memdump.NewMemoryDump(ctxData.Settings.PathToExecutable, ctxData.Settings.OutputPath, ctxData.Settings.DumpExecutablePath),
		}
	} else {
		return &Loop{
			Context: ctx,
			Output:  output.NewFilesystem(ctxData.Settings.OutputPath),
			Trace:   tm,
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

			mMsg := new(mutator.MutatedMessage)
			if err := mMsg.New(*message.Message, message.Descriptor, []string{}); err == nil {
				if _, err := mMsg.MutateMessage(); err != nil {
					log.Println(err)
				}
			}
			// resp := l.runIteration()
			// if resp != nil {
			// 	log.Printf("Got response: %s", resp)
			// }
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

func (l *Loop) getMesasageEnergyData(path string, data *string) (int, []trace.CoverageBlock, error) {
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

	_, err := communication.SendRequestWithMessage(req)
	if err != nil {
		return 0, nil, err
	}

	cov, err := l.Trace.GetCoverage()
	if err != nil {
		return 0, nil, errors.WithMessage(err, "Failed to get data for message energy calculation!")
	}

	t, err := l.Trace.GetLastExecTime()
	if err != nil {
		return 0, nil, errors.WithMessage(err, "Failed to get data for message energy calculation!")
	}

	if err := l.Trace.ClearCoverage(); err != nil {
		log.Println("Failed to clear coverage information. Next run coverage may contain garbage data!")
	}

	return t, cov, nil
}

func (l *Loop) initializeLoop() {
	loopData := l.Context.Value("data").(models.ContextData)
	messages := packet.GetParsedMessages(
		loopData.Settings.PcapFilePath,
		loopData.Settings.ProtoFilesPath,
		loopData.Settings.ProtoFilesIncludePath)

	if len(messages) == 0 {
		log.Fatal("No messages were processed! Bailing out...")
	}

	l.MsgDeps, l.MsgDepMap = packet.CalculateRelationMatrix(messages)
	l.ValueDeps = packet.CalculateReqResRelations(messages)
	l.prepareMessages(messages)

	if err := l.calculateMessagesEnergy(); err != nil {
		log.Printf("Error occured while initializing the Loop: %s", err)
	}

	if err := l.Events.NewEventManager(events.DefaultWindowsQuery); err != nil {
		log.Printf("Error occured while initializing the Loop: %s", err)
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

func (l *Loop) prepareMessages(msgs []packet.ProtoByteMsg) {
	uniqMsgs := packet.DistinctMessages(msgs)
	l.Messages = make([]LoopMessage, 0, 1)
	for _, msg := range uniqMsgs {
		l.Messages = append(l.Messages, LoopMessage{
			Path:       msg.Path,
			Message:    msg.Message,
			Descriptor: msg.Descriptor,
			Energy:     0,
			Coverage:   make([]trace.CoverageBlock, 0, 1),
		})
	}
}

func (l *Loop) calculateMessagesEnergy() error {
	loopData := l.Context.Value("data").(models.ContextData)
	procName := filepath.Base(loopData.Settings.PathToExecutable)

	var timeArr, covLenArr, fCountArr []int
	timeArr = make([]int, len(l.Messages))
	covLenArr = make([]int, len(l.Messages))
	fCountArr = make([]int, len(l.Messages))
	//go l.handleProcessStart()

	for i := 0; i < len(l.Messages); i++ {
		var hnd config.Handler
		for j := 0; j < len(loopData.Settings.Handlers); j++ {
			if loopData.Settings.Handlers[j].Method == l.Messages[i].Path {
				hnd = loopData.Settings.Handlers[j]
				break
			}
		}

		if err := l.Trace.Start(procName, hnd); err != nil {
			return errors.WithMessage(err, "Failed to start tracing session for the energy calculation!")
		}

		tExec, cov, _ := l.getMesasageEnergyData(l.Messages[i].Path, l.Messages[i].Message)
		l.Messages[i].Coverage = append(l.Messages[i].Coverage, cov...)
		timeArr[i] = tExec
		covLenArr[i] = len(cov)
		fCountArr[i] = packet.GetMessageFieldCount(l.Messages[i].Descriptor)

		if err := l.Trace.Unload(); err != nil {
			return errors.WithMessage(err, "Failed to unload script however energy calculation is finished!")
		}
	}

	if err := l.Trace.Stop(); err != nil {
		return errors.WithMessage(err, "Failed to stop tracing session however energy calculation is finished!")
	}

	util.ScaleIntegers(covLenArr, 1, 10)
	util.ScaleIntegers(fCountArr, 1, 10)
	util.ScaleIntegersReverse(timeArr, 1, 10)

	for i := 0; i < len(l.Messages); i++ {
		l.Messages[i].Energy += covLenArr[i]
		l.Messages[i].Energy += fCountArr[i]
		l.Messages[i].Energy += timeArr[i]
	}

	sort.Slice(l.Messages, func(i, j int) bool {
		return l.Messages[i].Energy > l.Messages[j].Energy
	})

	return nil
}

func (l *Loop) handleProcessStart() {
	loopData := l.Context.Value("data").(models.ContextData)
	startProgStatus := make(chan *watcher.StartProcessResponse)

	if !watcher.IsProcessRunning(l.Context) {
		go watcher.StartProcess(l.Context, startProgStatus)

		dumpPath := ""
		var err error

		if loopData.Settings.PerformMemoryDump {
			dumpPath, err = l.MemDump.StartDump()
			if err != nil {
				log.Printf("Failed to start the memory dump for the process: %s", err)
			}
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
