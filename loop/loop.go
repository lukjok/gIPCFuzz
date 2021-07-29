package loop

import (
	"context"
	"fmt"
	"log"

	"github.com/lukjok/gipcfuzz/communication"
	"github.com/lukjok/gipcfuzz/packet"
	"github.com/lukjok/gipcfuzz/util"
)

func Run(ctx context.Context) {
	loopData := ctx.Value("data").(LoopData)
	messages := packet.GetParsedMessages(
		loopData.Settings.PcapFilePath,
		loopData.Settings.ProtoFilesPath,
		loopData.Settings.ProtoFilesIncludePath)
	for _, message := range messages {
		RunIteration(ctx, message.Path, message.Message)
	}
}

func RunIteration(ctx context.Context, path string, data *string) {
	curIterData := ctx.Value("data").(LoopData)

	endpoint := fmt.Sprintf("%s:%d", curIterData.Settings.Host, curIterData.Settings.Port)
	protoFiles := util.GetFileFullPathInDirectory(curIterData.Settings.ProtoFilesPath, []string{"Includes"})
	req := communication.GIPCRequestWithMessage{
		Endpoint:          endpoint,
		Path:              path,
		Data:              data,
		ProtoFiles:        protoFiles,
		ProtoIncludesPath: curIterData.Settings.ProtoFilesIncludePath,
	}

	ret, err := communication.SendRequestWithMessage(req)
	if err != nil {
		log.Println("Error while sending request")
	}
	log.Println(ret)
}
