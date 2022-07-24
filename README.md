# gIPCFuzz

A grey box coverage feedback-based gRPC IPC fuzzer for desktop applications. I wrote this fuzzer from scratch for my master's thesis because why not. This is a prototype and it is not stable yet.

## How it works

The fuzzer accepts two mandatory input file types: network packet capture files (pcap, pcapng) and .proto definition files. Then it parses the gRPC HTTP/2 packages and tries to match all the packages with the definitions from the .proto files. After parsing the messages, the fuzzing stage can be started. The high-level structure of the fuzzer is presented below:

![gIPCFuzz structure](/images/structure.png)

Fuzzer accepts settings from a local JSON configuration file. The sample of the file is presented below:
```
{
    "pathToExecutable": "C:\\Demo\\Server\\server.exe",
    "executableArgs": [],
    "outputPath": "C:\\Demo\\Output",
    "dumpExecutablePath": "C:\\Demo\\procdump64.exe",
    "performMemoryDump": true,
    "handlers": [ 
        {"method": "test.testService/MethodOneBad", "module": "server.exe", "handler": "0x7A40"},
        {"method": "test.testService/MethodTwoBad", "module": "server.exe", "handler": "0xB490"},
        {"method": "test.testService/MethodThreeBad", "module": "server.exe", "handler": "0xAD00"},
        {"method": "test.testService/MethodFourBad", "module": "server.exe", "handler": "0x6900"},
        {"method": "test.testService/MethodFiveBad", "module": "server.exe", "handler": "0x6610"},
        {"method": "test.testService/MethodSixBad", "module": "server.exe", "handler": "0x8770"},
        {"method": "test.testService/MethodSevenBad", "module": "server.exe", "handler": "0x8100"},
        {"method": "test.testService/MethodEightBad", "module": "server.exe", "handler": "0x60F0"},
        {"method": "test.testService/MethodNinePartOne", "module": "server.exe", "handler": "0x6D10"},
        {"method": "test.testService/MethodNinePartTwoBad", "module": "server.exe", "handler": "0x6F80"},
        {"method": "test.testService/MethodTenPartOne", "module": "server.exe", "handler": "0x9700"},
        {"method": "test.testService/MethodTenPartTwo", "module": "server.exe", "handler": "0xA7C0"},
        {"method": "test.testService/MethodTenPartThreeBad", "module": "server.exe", "handler": "0x9970"},
        {"method": "test.testService/HtmlDocOne", "module": "server.exe", "handler": "0x356D0"},
        {"method": "test.testService/HtmlDocTwo", "module": "server.exe", "handler": "0x357C0"}
    ],
    "host": "localhost",
    "port": 50051,
    "ssl": false,
    "performDryRun": false,
    "singleFieldMutation": false,
    "dependencyUnawareSending": true,
	  "useInstrumentation": true,
    "maxMsgSize": 4194304,
    "protoFilesPath": "C:\\Demo\\Protos",
    "protoFilesIncludePath": ["C:\\Demo\\Protos"],
    "pcapFilePath": "C:\\Demo\\1.pcapng"
}
```

### Message sending

The fuzzer can send messages in two ways:
* Send single messages
* Send message chains

Sending a single message is pretty straightforward. On the other hand, sending message chains is a little bit more complex. Let's say there is a vulnerable function f3. This function depends on the other function's f1 and f2 results. You cannot fuzz f3 directly because the fuzzed application will reject the messages to this function. That's why this functionality was introduced. 

Message chain sending involves calculating probabilities in what order multiple messages are being sent. After that, the algorithm tries to recognize the message and message field dependencies by matching message field names and types. All this information is combined to make all possible message chains. However, only the last message of the chain is fuzzed. Other remaining messages needed to get a fuzzed application to the required state before reaching a vulnerable state.

### Coverage feedback

To obtain application fuzzed code coverage, Frida dynamic instrumentation library was used. Fuzzer is using Frida's Interceptor and Stalker functionality to gather the basic executed instruction blocks. The blocks are being collected after executing the fuzzed gRPC handler function. After that, the fuzzer tries to compare the currently recorded execution blocks with the new ones and if the difference is present, the currently fuzzed message (message chain) is added to the fuzzer message (message chain) queue.

![gIPCFuzz Frida usage](/images/frida.png)

### Message fuzzing cycles number calculation

Before the start of the fuzzing, the fuzzer will calculate the probable fuzzing cycle number for each message in the queue. This is being done because while fuzzing some messages might be more interesting than others. This is decided on these criteria:
* Message field count
* Application execution time while processing the message
* The number of executed basic instruction blocks while processing the message

The number of maximum cycles that the message can be fuzzed is calculated based on these criteria. Then the message queue is ordered to prioritize messages with the biggest cycle number. Calculations are performed with initial messages so some inconsistencies might occur if the application behaves differently based on the message content.

### Crash detection and corpus generation

Fuzzed application crashes are being detected when gRPC sending routines throw errors about unreachable fuzzing targets. In addition to this, when the fuzzed application crashes, the crash is handled by the fuzzer. After the crash, the fuzzer collect fuzzed application events from the OS Event Log and tries to get the stack trace of the application.

Currently, fuzzer can recognize two types of vulnerabilities: buffer overflow and null-pointer dereference vulnerabilities. This is done by checking the fuzzed application error code from the event log. This is not a very convenient solution since not all error codes mean the exact vulnerability. It can be improved in the future.

In addition to this, the fuzzer can also use the Sysinternals ProcDump tool to generate a memory dump file of the fuzzed process. This can be used in the further triaging process.

In the end, all the required information about the crash is saved in the JSON file. The example is provided below:
```
{
	"errorCode": "0xc0000005",
	"errorCause": "Null-pointer dereference",
	"moduleName": "server.exe",
	"faultFunction": "0x6F80",
	"methodPath": "test.testService/MethodNinePartTwoBad",
	"executableOutput": "",
	"executableEvents": [
		"Faulting application name: server.exe, version: 0.0.0.0, time stamp: 0x6252ff03\r\nFaulting module name: ntdll.dll, version: 10.0.19041.1806, time stamp: 0x1000a5b9\r\nException code: 0xc0000005\r\nFault offset: 0x00000000000269e2\r\nFaulting process id: 0x2fa4\r\nFaulting application start time: 0x01d89f5829786d3d\r\nFaulting application path: C:\\Demo\\Server\\server.exe\r\nFaulting module path: C:\\WINDOWS\\SYSTEM32\\ntdll.dll\r\nReport Id: 08e74f32-6d36-4b9f-83ee-81595e2a72bd\r\nFaulting package full name: \r\nFaulting package-relative application ID: "
	],
	"memoryDumpPath": "C:\\Demo\\Output\\server.exe_20220724152324",
	"crashMessage": "0ad8044c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c616261734c61626..."
}
```

### Current limitations
* Available only on Windows OS
* Frida feedback coverage is very unstable (frequent crashes)
* You cannot fuzz Golang binaries
* Fuzzer logic is pretty basic

### TODO
* Needs heavy refactoring and code cleaning
* Make it platform independent
* Replace Frida with other instrumentation
* ...
