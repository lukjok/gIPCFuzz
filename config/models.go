package config

type Configuration struct {
	PathToExecutable      string    `json:"pathToExecutable"`
	ExecutableArguments   []string  `json:"executableArgs"`
	OutputPath            string    `json:"outputPath"`
	DumpExecutablePath    string    `json:"dumpExecutablePath"`
	PerformMemoryDump     bool      `json:"performMemoryDump"`
	Handlers              []Handler `json:"handlers"`
	Host                  string    `json:"host"`
	Port                  int32     `json:"port"`
	SSL                   bool      `json:"ssl"`
	ProtoFilesPath        string    `json:"protoFilesPath"`
	ProtoFilesIncludePath []string  `json:"protoFilesIncludePath"`
	PcapFilePath          string    `json:"pcapFilePath"`
}

type Handler struct {
	Module      string `json:"module"`
	HandlerName string `json:"handler"`
}
