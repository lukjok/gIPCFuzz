package loop

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
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
	Logger         *util.Log
	Context        context.Context
	Messages       []LoopMessage
	MessageChains  []DependentMsgChain
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

	var logger *util.Log = util.NewLogger("log.txt")
	logger.LogInfo("Logger was initialized!")

	traceManager, tmError := trace.NewTraceManager()
	if tmError != nil {
		logger.LogError("Failed to initialize tracing manager!")
		os.Exit(1)
	}

	if ctxData.Settings.PerformMemoryDump {
		return &Loop{
			Logger:  logger,
			Context: ctx,
			Status:  &LoopStatus{},
			Output:  output.NewFilesystem(ctxData.Settings.OutputPath),
			Events:  &events.Events{},
			Trace:   traceManager,
			MemDump: memdump.NewMemoryDump(ctxData.Settings.PathToExecutable, ctxData.Settings.OutputPath, ctxData.Settings.DumpExecutablePath),
		}
	} else {
		return &Loop{
			Logger:  logger,
			Context: ctx,
			Status:  &LoopStatus{},
			Output:  output.NewFilesystem(ctxData.Settings.OutputPath),
			Trace:   traceManager,
			Events:  &events.Events{},
		}
	}
}

func (l *Loop) Run() {
	rSrc := rand.NewSource(time.Hour.Nanoseconds())
	loopData := l.Context.Value("data").(models.ContextData)
	l.initializeLoop()
	// // Initializing new trace manager since this helps to prevent Frida crashes
	// tm, err := trace.NewTraceManager()
	// if err != nil {
	// 	log.Fatalf("Failed to initialize tracing manager!")
	// }
	// l.Trace = tm
	if loopData.Settings.DoDependencyUnawareSending {
		l.doDependencyUnawareSending(rSrc)
	} else {
		l.doDependencyAwareSending(rSrc)
	}
}

func (l *Loop) doDependencyAwareSending(rSrc rand.Source) {
	l.Logger.LogInfo("Starting dependency aware sending!")
	loopData := l.Context.Value("data").(models.ContextData)
	for idx, mChain := range l.MessageChains {
		select {
		case <-l.Context.Done():
			l.Stop()
			return
		default:
			l.Status.IterationNo = idx + 1
			l.CurrentMessage = &mChain.Messages[len(mChain.Messages)-1]
			mutMgr := new(mutator.MutatorManager)

			var mutStrategy mutator.MutationStrategy
			if loopData.Settings.DoSingleFieldMutation {
				mutStrategy = mutator.SingleField
			}
			mutMgr.New(new(mutator.DefaultDependencyUnawareMut), new(mutator.DefaultDependencyAwareMut), int(loopData.Settings.MaxMsgSize), rSrc, []string{}, mutStrategy)

			if len(mChain.Messages) == 1 {
				continue
			}

			if len(l.CurrentMessage.Message) == 0 {
				continue
			}

			message := dynamic.NewMessage(l.CurrentMessage.Descriptor)
			if err := message.Unmarshal(l.CurrentMessage.Message); err != nil {
				l.Logger.LogError(err.Error())
				continue
			}

			ticker := time.NewTicker(1 * time.Second)
			for range ticker.C {
				if watcher.IsProcessRunning(l.Context) {
					break
				}
			}
			ticker.Stop()
			ticker = nil

			if len(mChain.Messages) > 1 {
				// Send all messages in order before the last one
				l.sendFirstChainMessages(mChain.Messages[:len(mChain.Messages)-1])
			}

			if loopData.Settings.UseInstrumentation {
				procName := filepath.Base(loopData.Settings.PathToExecutable)

				var hnd config.Handler
				for j := 0; j < len(loopData.Settings.Handlers); j++ {
					if loopData.Settings.Handlers[j].Method == l.CurrentMessage.Path {
						hnd = loopData.Settings.Handlers[j]
						break
					}
				}

				if err := l.Trace.Start(procName, hnd); err != nil {
					l.Logger.LogError(err.Error())
				}
			}

			// Parse dependent messages
			depMsgs := make([]dynamic.Message, 0, 1)
			for i := 0; i < len(mChain.DepMessages); i++ {
				// If bad dependent message appears, skip it (this should be a non existant case)
				if len(mChain.DepMessages[i].Message) == 0 {
					continue
				}

				message := dynamic.NewMessage(mChain.DepMessages[i].Descriptor)
				if err := message.Unmarshal(mChain.DepMessages[i].Message); err != nil {
					l.Logger.LogError(err.Error())
					continue
				}
				depMsgs = append(depMsgs, *message)
			}

			var programCrashed bool = false

			for i := l.CurrentMessage.Energy; i != 0; i-- {

				if programCrashed && len(mChain.Messages) > 1 {
					// Send all messages in order before the last one
					l.sendFirstChainMessages(mChain.Messages[:len(mChain.Messages)-1])
					programCrashed = false
				}

				err := mutMgr.DoAwareMutation(l.CurrentMessage.Descriptor, message, &l.CurrentMessage.Message, l.ValueDeps, depMsgs)
				if err != nil {
					l.Logger.LogError(err.Error())
					break
				}

				_, rErr := l.runIterationWithData(l.CurrentMessage.Path, l.CurrentMessage.Message)

				l.Status.MsgProg = 100 - float64((100*i)/l.CurrentMessage.Energy)
				l.Status.TotalExec += 1

				if rErr != nil {
					l.Logger.LogError(err.Error())
					l.handleIterationErr(rErr)
					programCrashed = true
				}

				if loopData.Settings.UseInstrumentation {
					cov, err := l.Trace.GetCoverage()
					if err != nil {
						l.Logger.LogError(err.Error())
					}

					l.processCoverageAndAppendMsg(cov)
					if err := l.Trace.ClearCoverage(); err != nil {
						l.Logger.LogError(err.Error())
					}
				}

				l.sendUIUpdate()
			}

			if loopData.Settings.UseInstrumentation {
				if err := l.Trace.Unload(); err != nil {
					l.Logger.LogError(err.Error())
				}
			}

			l.sendUIUpdate()
		}
	}
}

func (l *Loop) doDependencyUnawareSending(rSrc rand.Source) {
	l.Logger.LogInfo("Starting dependency aware sending!")
	loopData := l.Context.Value("data").(models.ContextData)
	for idx, message := range l.Messages {
		select {
		case <-l.Context.Done():
			l.Stop()
			return
		default:
			l.Status.IterationNo = idx + 1
			l.CurrentMessage = &message
			//mutMgr := new(mutator.MutatorManager)

			var mutStrategy mutator.MutationStrategy
			if loopData.Settings.DoSingleFieldMutation {
				mutStrategy = mutator.SingleField
			} else {
				mutStrategy = mutator.WholeMessage
			}
			//mutMgr.New(new(mutator.DefaultDependencyUnawareMut), new(mutator.DefaultDependencyAwareMut), rSrc, []string{}, mutStrategy)

			if len(l.CurrentMessage.Message) == 0 {
				continue
			}

			message := dynamic.NewMessage(l.CurrentMessage.Descriptor)
			if err := message.Unmarshal(l.CurrentMessage.Message); err != nil {
				l.Logger.LogError(err.Error())
				continue
			}

			ticker := time.NewTicker(1 * time.Second)
			for range ticker.C {
				if watcher.IsProcessRunning(l.Context) {
					break
				}
			}
			ticker.Stop()
			ticker = nil

			if loopData.Settings.UseInstrumentation {
				procName := filepath.Base(loopData.Settings.PathToExecutable)

				var hnd config.Handler
				for j := 0; j < len(loopData.Settings.Handlers); j++ {
					if loopData.Settings.Handlers[j].Method == l.CurrentMessage.Path {
						hnd = loopData.Settings.Handlers[j]
						break
					}
				}

				if err := l.Trace.Start(procName, hnd); err != nil {
					l.Logger.LogError(err.Error())
				}
			}

			for i := l.CurrentMessage.Energy; i != 0; i-- {
				mutMgr := new(mutator.MutatorManager)
				mutMgr.New(new(mutator.DefaultDependencyUnawareMut), new(mutator.DefaultDependencyAwareMut), int(loopData.Settings.MaxMsgSize), rSrc, []string{}, mutStrategy)

				err := mutMgr.DoMutation(l.CurrentMessage.Descriptor, message, &l.CurrentMessage.Message)
				if err != nil {
					message = nil
					l.Logger.LogError(err.Error())
					break
				}

				_, rErr := l.runIterationWithData(l.CurrentMessage.Path, l.CurrentMessage.Message)

				l.Status.MsgProg = 100 - float64((100*i)/l.CurrentMessage.Energy)
				l.Status.TotalExec += 1

				if rErr != nil {
					l.Logger.LogError(rErr.Error())
					l.handleIterationErr(rErr)
				}

				if loopData.Settings.UseInstrumentation {
					cov, err := l.Trace.GetCoverage()
					if err != nil {
						l.Logger.LogError(err.Error())
					}

					l.processCoverageAndAppendMsg(cov)
					if err := l.Trace.ClearCoverage(); err != nil {
						l.Logger.LogError(err.Error())
					}
				}

				l.sendUIUpdate()
				runtime.GC()
				mutMgr = nil
			}

			if loopData.Settings.UseInstrumentation {
				if err := l.Trace.Unload(); err != nil {
					l.Logger.LogError(err.Error())
				}
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
	loopData := l.Context.Value("data").(models.ContextData)
	if loopData.Settings.UseInstrumentation {
		if err := l.Trace.Unload(); err != nil {
			l.Logger.LogError(err.Error())
		}
		if err := l.Trace.Stop(); err != nil {
			l.Logger.LogError(err.Error())
		}
	}
	l.Events.StopCapture()
}

func (l *Loop) sendFirstChainMessages(msgs []LoopMessage) error {
	for i := 0; i < len(msgs); i++ {
		if _, err := l.runIterationWithData(msgs[i].Path, msgs[i].Message); err != nil {
			return errors.WithMessage(err, "Error occured while sending chain message!")
		}
	}
	return nil
}

func (l *Loop) runIterationWithData(path string, data []byte) (protoiface.MessageV1, error) {
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

func (l *Loop) getMesasageEnergyData(path string, data []byte) (int, []trace.CoverageBlock, error) {
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

func (l *Loop) getMesasageChainEnergyData(msgChain DependentMsgChain, handler config.Handler) (int, []trace.CoverageBlock, error) {
	curIterData := l.Context.Value("data").(models.ContextData)
	endpoint := fmt.Sprintf("%s:%d", curIterData.Settings.Host, curIterData.Settings.Port)
	protoFiles := util.GetFileFullPathInDirectory(curIterData.Settings.ProtoFilesPath, []string{"Includes"})
	procName := filepath.Base(curIterData.Settings.PathToExecutable)

	if len(msgChain.Messages) == 1 {
		if err := l.Trace.Start(procName, handler); err != nil {
			return 0, nil, errors.WithMessage(err, "Failed to start tracing session for the energy calculation!")
		}

		t, cov, err := l.getMesasageEnergyData(msgChain.Messages[0].Path, msgChain.Messages[0].Message)
		if err != nil {
			return t, cov, errors.WithMessage(err, "Failed to perform energy calculation!")
		}

		if err := l.Trace.Unload(); err != nil {
			return t, cov, errors.WithMessage(err, "Failed to unload script however energy calculation is finished!")
		}
		return t, cov, nil
	}

	for i := 0; i < len(msgChain.Messages)-1; i++ {
		if _, err := l.runIterationWithData(msgChain.Messages[i].Path, msgChain.Messages[i].Message); err != nil {
			return 0, nil, errors.WithMessage(err, "Error occured while sending trailing chain message!")
		}
	}

	lastMsg := msgChain.Messages[len(msgChain.Messages)-1]

	req := communication.GIPCRequest{
		Endpoint:          endpoint,
		Path:              lastMsg.Path,
		Data:              lastMsg.Message,
		ProtoFiles:        protoFiles,
		ProtoIncludesPath: curIterData.Settings.ProtoFilesIncludePath,
	}

	if err := l.Trace.Start(procName, handler); err != nil {
		return 0, nil, errors.WithMessage(err, "Failed to start tracing session for the energy calculation!")
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

	if err := l.Trace.Unload(); err != nil {
		return t, cov, errors.WithMessage(err, "Failed to unload script however energy calculation is finished!")
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
		l.Logger.LogError("No messages were processed! Bailing out...")
		os.Exit(1)
	}

	if loopData.Settings.DoDependencyUnawareSending {
		l.prepareMessages(messages)

		if err := l.calculateMessagesEnergy(); err != nil {
			l.Logger.LogError(err.Error())
		}
	} else {
		l.MsgDeps, l.MsgDepMap = packet.CalculateRelationMatrix(messages)
		l.ValueDeps = packet.CalculateReqResRelations(messages)
		l.prepareMessageChains(messages)

		if err := l.calculateMessageChainEnergy(); err != nil {
			l.Logger.LogError(err.Error())
		}
	}

	if err := l.Events.NewEventManager(events.DefaultWindowsQuery); err != nil {
		l.Logger.LogError(err.Error())
	}

	//l.Trace.Cleanup() // Clean and free any existing Frida objects before creating new instance
	l.Events.StartCapture()
	go l.handleProcessStart()

	if loopData.Settings.DryRun {
		l.Logger.LogInfo("Performing a dry run before starting a fuzzing process...")
		if err := l.performDryRun(); err != nil {
			l.Logger.LogError("Dry run enabled and the process failed to respond to the valid message! Bailing out")
			os.Exit(1)
		}
	}
}

func (l *Loop) prepareMessageChains(msgs []packet.ProtoByteMsg) {
	msgChains := make([]DependentMsgChain, 0, 1)
	rMsgChains := make([][]string, 0, 1)
	// Calculate longest chain and then make shorter ones by reducing messages by one
	// Adapt it if chain is broken: finish one chain creation and start another one
	var lastMsgIdx int = 0
	var isChainBroken bool = true
	longestChain := make([]string, 0, 1)
	for i := 0; i < len(l.MsgDeps); i++ {
		isChainBroken = true
		for j := 0; j < len(l.MsgDeps); j++ {
			if l.MsgDeps[i][j] >= 0.5 && i != j {
				name1 := util.GetMapKeyByValue(l.MsgDepMap, i)
				longestChain = append(longestChain, name1)
				lastMsgIdx = j
				isChainBroken = false
			}
		}
		if isChainBroken {
			// Add last message of the chain
			lastName := util.GetMapKeyByValue(l.MsgDepMap, lastMsgIdx)
			longestChain = append(longestChain, lastName)
			rMsgChains = append(rMsgChains, longestChain)
			longestChain = nil
		}
	}
	// Add last message of the chain
	lastName := util.GetMapKeyByValue(l.MsgDepMap, lastMsgIdx)
	longestChain = append(longestChain, lastName)
	rMsgChains = append(rMsgChains, longestChain)
	longestChain = nil

	for i := 0; i < len(rMsgChains); i++ {
		for j := 0; j < len(rMsgChains[i]); j++ {
			lMsgs := make([]LoopMessage, 0, 1)
			for k := 0; k < j+1; k++ {
				if pbMsg := getMessageByPathName(msgs, rMsgChains[i][k]); pbMsg != nil {
					msgBuf, _ := hex.DecodeString(*pbMsg.Message)
					lMsgs = append(lMsgs, LoopMessage{
						Path:       pbMsg.Path,
						Message:    msgBuf,
						Descriptor: pbMsg.Descriptor,
						Energy:     0,
						Coverage:   make([]trace.CoverageBlock, 0, 1),
					})
				}
			}
			dMsgs := make([]LoopMessage, 0, 1)
			lastMsgName := lMsgs[len(lMsgs)-1].Descriptor.GetName()
			for i := 0; i < len(l.ValueDeps); i++ {
				var neededmsgName string = ""
				if l.ValueDeps[i].Msg1 == lastMsgName {
					neededmsgName = l.ValueDeps[i].Msg2
				}
				if l.ValueDeps[i].Msg2 == lastMsgName {
					neededmsgName = l.ValueDeps[i].Msg1
				}
				if len(neededmsgName) > 0 {
					if pbMsg := getMessageByPathNamesInOrder(msgs, neededmsgName, lastMsgName); pbMsg != nil {
						msgBuf, _ := hex.DecodeString(*pbMsg.Message)
						dMsgs = append(dMsgs, LoopMessage{
							Path:       pbMsg.Path,
							Message:    msgBuf,
							Descriptor: pbMsg.Descriptor,
							Energy:     0,
							Coverage:   make([]trace.CoverageBlock, 0, 1),
						})
					}
				}
			}
			msgChains = append(msgChains, DependentMsgChain{
				Energy:      0,
				Messages:    lMsgs,
				DepMessages: dMsgs,
			})
		}
	}
	l.MessageChains = msgChains
}

func getMessageByPathName(msgs []packet.ProtoByteMsg, path string) *packet.ProtoByteMsg {
	for i := 0; i < len(msgs); i++ {
		if msgs[i].Path == path {
			return &msgs[i]
		}
	}
	return nil
}

func getMessageByPathNamesInOrder(msgs []packet.ProtoByteMsg, name1 string, name2 string) *packet.ProtoByteMsg {
	for i := 0; i < len(msgs)-1; i++ {
		if msgs[i].Descriptor.GetName() == name1 && msgs[i+1].Descriptor.GetName() == name2 {
			return &msgs[i]
		}
	}
	return nil
}

func (l *Loop) prepareMessages(msgs []packet.ProtoByteMsg) {
	uniqMsgs := packet.DistinctMessages(msgs)
	l.Messages = make([]LoopMessage, 0, 1)
	for _, msg := range uniqMsgs {
		msgBuf, _ := hex.DecodeString(*msg.Message)
		l.Messages = append(l.Messages, LoopMessage{
			Path:       msg.Path,
			Message:    msgBuf,
			Descriptor: msg.Descriptor,
			Energy:     0,
			Coverage:   make([]trace.CoverageBlock, 0, 1),
		})
	}
}

func (l *Loop) calculateMessageChainEnergy() error {
	loopData := l.Context.Value("data").(models.ContextData)

	var timeArr, covLenArr, fCountArr []int
	timeArr = make([]int, len(l.MessageChains))
	covLenArr = make([]int, len(l.MessageChains))
	fCountArr = make([]int, len(l.MessageChains))
	go l.handleProcessStartWithoutReporting()

	for i := 0; i < len(l.MessageChains); i++ {
		var hnd config.Handler
		for j := 0; j < len(loopData.Settings.Handlers); j++ {
			if loopData.Settings.Handlers[j].Method == l.MessageChains[i].Messages[len(l.MessageChains[i].Messages)-1].Path {
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
		ticker = nil

		tExec, cov, _ := l.getMesasageChainEnergyData(l.MessageChains[i], hnd)
		l.MessageChains[i].Messages[len(l.MessageChains[i].Messages)-1].Coverage = append(l.MessageChains[i].Messages[len(l.MessageChains[i].Messages)-1].Coverage, cov...)
		timeArr[i] = tExec
		covLenArr[i] = len(cov)
		fCountArr[i] = packet.GetMessageFieldCount(l.MessageChains[i].Messages[len(l.MessageChains[i].Messages)-1].Descriptor)
	}

	if err := l.Trace.Stop(); err != nil {
		return errors.WithMessage(err, "Failed to stop tracing session however energy calculation is finished!")
	}

	util.ScaleIntegers(covLenArr, 1, 10)
	util.ScaleIntegers(fCountArr, 1, 10)
	util.ScaleIntegersReverse(timeArr, 1, 10)

	for i := 0; i < len(l.MessageChains); i++ {
		l.MessageChains[i].Energy += covLenArr[i]
		l.MessageChains[i].Energy += fCountArr[i]
		l.MessageChains[i].Energy += timeArr[i]
		l.MessageChains[i].Messages[len(l.MessageChains[i].Messages)-1].Energy = l.MessageChains[i].Energy
	}

	sort.Slice(l.MessageChains, func(i, j int) bool {
		return l.MessageChains[i].Energy > l.MessageChains[j].Energy
	})

	watcher.KillProcess(l.Context) //Do cleanup after the energy calculation and kill the process

	return nil
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
		ticker = nil

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
				l.Logger.LogError(err.Error())
			}
		}

		for done := false; !done; {
			select {
			case <-l.Context.Done():
				close(startProgStatus)
				done = true
			case response := <-startProgStatus:
				if response != nil && response.Error != nil {
					l.Logger.LogError(response.Error.Error())
					done = true
				}
				if response != nil && response.Error == nil && len(response.Output) > 0 && response.Output != "EXIT" && l.CurrentMessage != nil {
					l.Status.LastCrashTime = time.Now()
					l.Status.UniqueCrashCount += 1 //TODO: it's unique crash count, so we need to calculate how many UNIQUE crashes occured
					l.writeIterationCrash(response.Output, dumpPath, l.CurrentMessage.Message)
					done = true
				}
				if response != nil && response.Error == nil && response.Output == "EXIT" && l.CurrentMessage != nil {
					// TODO: something is wrong, need to check
					l.Status.LastCrashTime = time.Now()
					l.Status.UniqueCrashCount += 1 //TODO: it's unique crash count, so we need to calculate how many UNIQUE crashes occured
					l.writeIterationCrash("", dumpPath, l.CurrentMessage.Message)
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
					l.Logger.LogError(response.Error.Error())
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

func (l *Loop) writeIterationCrash(processOutput, memoryDumpPath string, lastMessage []byte) {
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
		CrashMessage:     fmt.Sprintf("%x", lastMessage),
	}
	if methodHandler := util.GetMethodHandler(l.CurrentMessage.Path, loopData.Settings.Handlers); methodHandler != nil {
		crashOutput.ModuleName = methodHandler.Module
		crashOutput.FaultFunction = methodHandler.HandlerName
	}

	if len(processOutput) > 0 {
		crashOutput.ErrorCode = watcher.ParseErrorCode(processOutput)
		crashOutput.ErrorCause = watcher.ExplainErrorCode(crashOutput.ErrorCode)
	} else {
		if len(events) > 0 {
			crashOutput.ErrorCode = watcher.ParseErrorCode(events[0])
			crashOutput.ErrorCause = watcher.ExplainErrorCode(crashOutput.ErrorCode)
		}
	}

	if err := l.Output.SaveCrash(&crashOutput); err != nil {
		l.Logger.LogError(err.Error())
	}
}

func (l *Loop) handleIterationErr(err error) {
	convertedErr := util.ConvertError(err)
	if convertedErr == models.NetworkError {
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
