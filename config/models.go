package config

type Configuration struct {
	ProcessName            string   `json:"processName"`
	HandlerFunctionAddress uint     `json:"handlerFunctionAddress"`
	Host                   string   `json:"host"`
	Port                   int32    `json:"port"`
	SSL                    bool     `json:"ssl"`
	Modules                []string `json:"modules"`
	ProtoFilesPath         string   `json:"protoFilesPath"`
	ProtoFilesIncludePath  []string `json:"protoFilesIncludePath"`
	PcapFilePath           string   `json:"pcapFilePath"`
}
