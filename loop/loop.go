package loop

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"sort"
	"time"

	"github.com/jhump/protoreflect/dynamic"
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
	Status         *LoopStatus
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
			Status:  &LoopStatus{},
			Output:  output.NewFilesystem(ctxData.Settings.OutputPath),
			Events:  &events.Events{},
			Trace:   tm,
			MemDump: memdump.NewMemoryDump(ctxData.Settings.PathToExecutable, ctxData.Settings.OutputPath, ctxData.Settings.DumpExecutablePath),
		}
	} else {
		return &Loop{
			Context: ctx,
			Status:  &LoopStatus{},
			Output:  output.NewFilesystem(ctxData.Settings.OutputPath),
			Trace:   tm,
			Events:  &events.Events{},
		}
	}
}

func (l *Loop) Run() {
	rSrc := rand.NewSource(time.Hour.Nanoseconds())
	loopData := l.Context.Value("data").(models.ContextData)
	l.initializeLoop()

	for idx, message := range l.Messages {
		select {
		case <-l.Context.Done():
			l.Stop()
			return
		default:
			time.Sleep(2 * time.Second)
			l.Status.IterationNo = idx + 1
			l.CurrentMessage = &message
			mutMgr := new(mutator.MutatorManager)
			procName := filepath.Base(loopData.Settings.PathToExecutable)
			mutMgr.New(new(mutator.DefaultDependencyUnawareMut), new(mutator.DefaultDependencyAwareMut), rSrc, []string{})

			if len(*l.CurrentMessage.Message) == 0 {
				continue
			}

			buf, err := hex.DecodeString(*l.CurrentMessage.Message)
			if err != nil {
				continue
			}
			message := dynamic.NewMessage(l.CurrentMessage.Descriptor)
			if err := message.Unmarshal(buf); err != nil {
				continue
			}

			var hnd config.Handler
			for j := 0; j < len(loopData.Settings.Handlers); j++ {
				if loopData.Settings.Handlers[j].Method == l.CurrentMessage.Path {
					hnd = loopData.Settings.Handlers[j]
					break
				}
			}

			ticker := time.NewTicker(1 * time.Second)
			for range ticker.C {
				if watcher.IsProcessRunning(l.Context) {
					break
				}
			}
			ticker.Stop()

			if err := l.Trace.Start(procName, hnd); err != nil {
				//log.Println(err, "Failed to start tracing session for fuzzed message!")
			}

			for i := l.CurrentMessage.Energy; i != 0; i-- {
				mutMsg, err := mutMgr.DoSingleMessageMutation(l.CurrentMessage.Descriptor, message)
				if err != nil {
					break
				}

				l.CurrentMessage.Message = &mutMsg
				_, rErr := l.runIterationWithData(l.CurrentMessage.Path, &mutMsg)

				l.Status.MsgProg = 100 - float64((100*i)/l.CurrentMessage.Energy)
				l.Status.TotalExec += 1

				if rErr != nil {
					l.handleIterationErr(rErr)
				}

				cov, err := l.Trace.GetCoverage()
				if err != nil {
					//log.Println(err, "Failed to get fuzzed message coverage!")
				}

				// t, err := l.Trace.GetLastExecTime()
				// if err != nil {
				// 	log.Println(err, "Failed to get fuzzed message execution time!")
				// }

				if err := l.Trace.ClearCoverage(); err != nil {
					//log.Println("Failed to clear coverage information. Next run coverage may contain garbage data!")
				}

				l.processCoverageAndAppendMsg(cov)
				l.sendUIUpdate()
			}

			if err := l.Trace.Unload(); err != nil {
				//log.Println(err, "Failed to unload script")
			}

			l.sendUIUpdate()
		}
	}
}

func (l *Loop) sendUIUpdate() {
	loopData := l.Context.Value("data").(models.ContextData)
	loopData.UIDataChan <- &models.UIData{
		StartTime:           time.Time{},
		LastCrashTime:       l.Status.LastCrashTime,
		LastHangTime:        l.Status.LastHangTime,
		CyclesDone:          l.Status.IterationNo,
		TotalPaths:          l.Status.NewPathCount,
		ExecSpd:             l.Status.TotalExec / 60,
		UniqCrash:           l.Status.UniqueCrashCount,
		UniqHangs:           l.Status.UniqueHangCount,
		TotalExec:           l.Status.TotalExec,
		CurrMsg:             l.CurrentMessage.Path,
		MsgProg:             l.Status.MsgProg,
		MessageCountInQueue: len(l.Messages),
	}
}

func (l *Loop) processCoverageAndAppendMsg(cov []trace.CoverageBlock) {
	covChange := false
	if len(cov) != len(l.CurrentMessage.Coverage) {
		covChange = true
	} else {
		for i := 0; i < len(l.CurrentMessage.Coverage); i++ {
			if cov[i].BlockStart != l.CurrentMessage.Coverage[i].BlockStart || cov[i].BlockEnd != l.CurrentMessage.Coverage[i].BlockEnd {
				covChange = true
			}
		}
	}
	if !covChange {
		return
	}
	for i := 0; i < len(l.Messages); i++ {
		if len(cov) != len(l.Messages[i].Coverage) {
			//TODO: need to implement a energy recalculation method against existing messages
			l.Messages = append(l.Messages, LoopMessage{
				Path:       l.CurrentMessage.Path,
				Message:    l.CurrentMessage.Message,
				Descriptor: l.CurrentMessage.Descriptor,
				Energy:     l.CurrentMessage.Energy,
				Coverage:   cov,
			})
			l.Status.NewPathCount += 1
			l.Status.NewPathTime = time.Now()
			return
		} else {
			covChange = false
			for j := 0; j < len(l.Messages[i].Coverage) && !covChange; j++ {
				if cov[j].BlockStart != l.Messages[i].Coverage[j].BlockStart || cov[j].BlockEnd != l.Messages[i].Coverage[j].BlockEnd {
					covChange = true
				}
			}
			l.Messages = append(l.Messages, LoopMessage{
				Path:       l.CurrentMessage.Path,
				Message:    l.CurrentMessage.Message,
				Descriptor: l.CurrentMessage.Descriptor,
				Energy:     l.CurrentMessage.Energy,
				Coverage:   cov,
			})
			l.Status.NewPathCount += 1
			l.Status.NewPathTime = time.Now()
			return
		}
	}
}

func (l *Loop) Stop() {
	//log.Println("Recieved interrupt signal. Cleaning up...")
	if err := l.Trace.Unload(); err != nil {
		log.Println(err, "Failed to unload script!")
	}
	if err := l.Trace.Stop(); err != nil {
		log.Println(err, "Failed to stop tracing session!")
	}
	l.Events.StopCapture()
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
	go l.handleProcessStartWithoutReporting()

	for i := 0; i < len(l.Messages); i++ {
		var hnd config.Handler
		for j := 0; j < len(loopData.Settings.Handlers); j++ {
			if loopData.Settings.Handlers[j].Method == l.Messages[i].Path {
				hnd = loopData.Settings.Handlers[j]
				break
			}
		}

		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			if watcher.IsProcessRunning(l.Context) {
				break
			}
		}
		ticker.Stop()
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

	watcher.KillProcess(l.Context) //Do cleanup after the energy calculation and kill the process

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
					fmt.Println(response.Output)
					//("Error occurred while starting the process: %s", response.Error)
					done = true
				}
				if response != nil && response.Error == nil && len(response.Output) > 0 && response.Output != "EXIT" {
					l.Status.LastCrashTime = time.Now()
					l.Status.UniqueCrashCount += 1 //TODO: it's unique crash count, so we need to calculate how many UNIQUE crashes occured
					l.writeIterationCrash(response.Output, dumpPath, *l.CurrentMessage.Message)
					done = true
				}
				if response != nil && response.Error == nil && response.Output == "EXIT" {
					l.Status.LastCrashTime = time.Now()
					l.Status.UniqueCrashCount += 1 //TODO: it's unique crash count, so we need to calculate how many UNIQUE crashes occured
					l.writeIterationCrash("", dumpPath, *l.CurrentMessage.Message)
					done = true
				}
			default:
			}
		}
	}
}

func (l *Loop) handleProcessStartWithoutReporting() {
	startProgStatus := make(chan *watcher.StartProcessResponse)
	if !watcher.IsProcessRunning(l.Context) {
		go watcher.StartProcess(l.Context, startProgStatus)

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
				if response != nil && response.Error == nil && len(response.Output) > 0 && response.Output != "EXIT" {
					done = true
				}
				if response != nil && response.Error == nil && response.Output == "EXIT" {
					done = true
				}
			default:
			}
		}
	}
}

func (l *Loop) writeIterationCrash(processOutput, memoryDumpPath, lastMessage string) {
	loopData := l.Context.Value("data").(models.ContextData)
	events := l.Events.GetEventData()
	crashOutput := output.CrashOutput{
		ErrorCode:        "",
		ErrorCause:       "",
		ModuleName:       "",
		FaultFunction:    "",
		MethodPath:       l.CurrentMessage.Path,
		ExecutableOutput: processOutput,
		ExecutableEvents: events,
		MemoryDumpPath:   memoryDumpPath,
		CrashMessage:     lastMessage,
	}
	if methodHandler := util.GetMethodHandler(l.CurrentMessage.Path, loopData.Settings.Handlers); methodHandler != nil {
		crashOutput.ModuleName = methodHandler.Module
		crashOutput.FaultFunction = methodHandler.HandlerName
	}
	// TODO: This only performs check in the error output, not event logs, so this also can be a posibility
	crashOutput.ErrorCode = watcher.ParseErrorCode(processOutput)
	crashOutput.CrashMessage = watcher.ExplainErrorCode(crashOutput.ErrorCode)

	if err := l.Output.SaveCrash(&crashOutput); err != nil {
		log.Printf("Failed to write iteration crash dump with error: %s", err)
	}
}

func (l *Loop) handleIterationErr(err error) {
	convertedErr := util.ConvertError(err)
	if convertedErr == models.NetworkError {
		//log.Printf("gRPC request failed with error: %s", err)
		if !watcher.IsProcessRunning(l.Context) {
			go l.handleProcessStart()

			ticker := time.NewTicker(1 * time.Second)
			for range ticker.C {
				if watcher.IsProcessRunning(l.Context) {
					break
				}
			}
			ticker.Stop()
			l.Status.LastCrashTime = time.Now()
			l.Status.UniqueCrashCount += 1 //TODO: it's unique crash count, so we need to calculate how many UNIQUE crashes occured
		} else {
			l.Status.LastHangTime = time.Now()
			l.Status.UniqueHangCount += 1 //TODO: it's unique hang count, so we need to calculate how many UNIQUE hangs occured
		}
	}
	if convertedErr == models.GRPCError {
		//log.Printf("gRPC request failed with error: %s", err)
		if !watcher.IsProcessRunning(l.Context) {
			go l.handleProcessStart()

			ticker := time.NewTicker(1 * time.Second)
			for range ticker.C {
				if watcher.IsProcessRunning(l.Context) {
					break
				}
			}
			ticker.Stop()
			l.Status.LastCrashTime = time.Now()
			l.Status.UniqueCrashCount += 1 //TODO: it's unique crash count, so we need to calculate how many UNIQUE crashes occured
		} else {
			l.Status.LastHangTime = time.Now()
			l.Status.UniqueHangCount += 1 //TODO: it's unique hang count, so we need to calculate how many UNIQUE hangs occured
		}
	}
	if convertedErr == models.UnknownError {
		//log.Printf("gRPC request failed with error: %s", err)
		if !watcher.IsProcessRunning(l.Context) {
			go l.handleProcessStart()

			ticker := time.NewTicker(1 * time.Second)
			for range ticker.C {
				if watcher.IsProcessRunning(l.Context) {
					break
				}
			}
			ticker.Stop()
			l.Status.LastCrashTime = time.Now()
			l.Status.UniqueCrashCount += 1 //TODO: it's unique crash count, so we need to calculate how many UNIQUE crashes occured
		}
	}
}

func (l *Loop) performDryRun() error {
	sampleMessage := l.Messages[0]
	_, err := l.runIterationWithData(sampleMessage.Path, sampleMessage.Message)
	return err
}
