package loop

import (
	"context"
	"fmt"
	"log"

	"github.com/lukjok/gipcfuzz/communication"
	"github.com/lukjok/gipcfuzz/models"
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
	Context  context.Context
	Messages []packet.ProtoByteMsg
}

func (l *Loop) Run() {
	l.initializeLoop()
	for _, message := range l.Messages {
		resp, err := runIteration(l.Context, message.Path, message.Message)
		handleIterationErr(l.Context, err)
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
	go handleProcessStart(l.Context)
}

func handleProcessStart(ctx context.Context) {
	startProgStatus := make(chan watcher.StartProcessResponse)
	if !watcher.IsProcessRunning(ctx) {
		go watcher.StartProcess(ctx, startProgStatus)
		// select {
		// case response := <-startProgStatus:
		// 	if response.Error != nil {
		// 		log.Printf("Error occurred while starting the process: %s", response.Error)
		// 		break
		// 	}
		// 	fmt.Print(response.Output)
		// 	break
		// default:
		// 	log.Println("Channel default case")
		// }
		for done := false; !done; {
			select {
			case response := <-startProgStatus:
				if response.Error != nil {
					log.Printf("Error occurred while starting the process: %s", response.Error)
					break
				}
				fmt.Print(response.Output)
				break
			default:
				log.Println("Channel default case")
			}
		}
	}
}

func handleIterationErr(ctx context.Context, err error) {
	convertedErr := util.ConvertError(err)
	if convertedErr == models.NetworkError {
		log.Printf("gRPC request failed with error: %s", err)
		if !watcher.IsProcessRunning(ctx) {
			//TODO: Collect crash information if possible and restart the program
			// if !watcher.StartProcess(ctx) {
			// 	log.Print("Failed to start a specified program")
			// } else {
			// 	//time.Sleep(5 * time.Second)
			// 	if watcher.IsProcessRunning(ctx) {
			// 		return
			// 	}
			// }
		}
	}
}
