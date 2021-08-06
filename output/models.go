package output

type CrashOutput struct {
	IterationNo      int32    `json:"IterationNo"`
	MethodPath       string   `json:"methodPath"`
	ExecutableOutput string   `json:"executableOutput"`
	ExecutableEvents []string `json:"executableEvents"`
	MemoryDumpPath   string   `json:"memoryDumpPath"`
}

type IterationProgress struct {
	CurrentIteration int32  `json:"currentIteration"`
	CurrentMessage   string `json:"currentMessage"`
}
