package config

type Configuration struct {
	PathToExecutable       string   `json:"pathToExecutable"`
	ExecutableArguments    []string `json:"executableArgs"`
	OutputPath             string   `json:"outputPath"`
	DumpExecutablePath     string   `json:"dumpExecutablePath"`
	PerformMemoryDump      bool     `json:"performMemoryDump"`
	HandlerFunctionAddress uint     `json:"handlerFunctionAddress"`
	Host                   string   `json:"host"`
	Port                   int32    `json:"port"`
	SSL                    bool     `json:"ssl"`
	Modules                []string `json:"modules"`
	ProtoFilesPath         string   `json:"protoFilesPath"`
	ProtoFilesIncludePath  []string `json:"protoFilesIncludePath"`
	PcapFilePath           string   `json:"pcapFilePath"`
}
