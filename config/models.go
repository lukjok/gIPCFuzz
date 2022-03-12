package config

type Configuration struct {
	PathToExecutable           string    `json:"pathToExecutable"`
	ExecutableArguments        []string  `json:"executableArgs"`
	OutputPath                 string    `json:"outputPath"`
	DumpExecutablePath         string    `json:"dumpExecutablePath"`
	PerformMemoryDump          bool      `json:"performMemoryDump"`
	Handlers                   []Handler `json:"handlers"`
	Host                       string    `json:"host"`
	Port                       int32     `json:"port"`
	SSL                        bool      `json:"ssl"`
	DryRun                     bool      `json:"performDryRun"`
	DoSingleFieldMutation      bool      `json:"singleFieldMutation"`
	DoDependencyUnawareSending bool      `json:"dependencyUnawareSending"`
	UseInstrumentation         bool      `json:"useInstrumentation"`
	ProtoFilesPath             string    `json:"protoFilesPath"`
	ProtoFilesIncludePath      []string  `json:"protoFilesIncludePath"`
	PcapFilePath               string    `json:"pcapFilePath"`
}

type Handler struct {
	Method      string `json:"method"`
	Module      string `json:"module"`
	HandlerName string `json:"handler"`
}
