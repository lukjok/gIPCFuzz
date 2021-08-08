package output

type CrashOutput struct {
	IterationNo      int      `json:"IterationNo"`
	MethodPath       string   `json:"methodPath"`
	ExecutableOutput string   `json:"executableOutput"`
	ExecutableEvents []string `json:"executableEvents"`
	MemoryDumpPath   string   `json:"memoryDumpPath"`
}

type IterationProgress struct {
	CurrentIteration int    `json:"currentIteration"`
	CurrentMessage   string `json:"currentMessage"`
}
