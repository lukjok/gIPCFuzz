package output

type CrashOutput struct {
	ErrorCode        string   `json:"errorCode"`
	ErrorCause       string   `json:"errorCause"`
	ModuleName       string   `json:"moduleName"`
	FaultFunction    string   `json:"faultFunction"`
	MethodPath       string   `json:"methodPath"`
	ExecutableOutput string   `json:"executableOutput"`
	ExecutableEvents []string `json:"executableEvents"`
	MemoryDumpPath   string   `json:"memoryDumpPath"`
	CrashMessage     string   `json:"crashMessage"`
}

type IterationProgress struct {
	CurrentIteration int    `json:"currentIteration"`
	CurrentMessage   string `json:"currentMessage"`
}
