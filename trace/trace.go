package trace

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	frida_go "github.com/a97077088/frida-go"
	jsoniter "github.com/json-iterator/go"
	"github.com/lukjok/gipcfuzz/config"
	"github.com/lukjok/gipcfuzz/util"
	"github.com/pkg/errors"
)

const (
	scriptFilePath = "C:\\Source\\gIPCFuzz\\script.js"
)

type TraceManager interface {
	Start(string, []config.Handler) error
	GetCoverage() ([]CoverageBlock, error)
	ClearCoverage() error
	Stop() error
	Unload() error
}

type Trace struct {
	manager *frida_go.DeviceManager
	device  *frida_go.Device
	script  *frida_go.Script
	session *frida_go.Session
}

func NewTraceManager() (*Trace, error) {
	manager := frida_go.DeviceManager_Create()
	devices, err := manager.EnumerateDevices()

	if err != nil {
		log.Printf("Failed to enumerate devices: %s", err)
		return nil, errors.Errorf("Failed to enumerate devices: %s", err)
	}

	if len(devices) == 0 {
		return nil, errors.New("No devices found!")
	}

	return &Trace{
		manager: manager,
		device:  devices[0],
	}, nil
}

func (t *Trace) Start(pName string, handler config.Handler) error {
	p, err := t.device.GetProcessByName(pName, frida_go.ProcessMatchOptions{})
	if err != nil {
		return errors.Errorf("Failed to find specified process: %s", err)
	}

	if t.session == nil {
		t.session, err = t.device.Attach(p.Pid(), frida_go.SessionOptions{})
		if err != nil {
			return errors.Errorf("Failed to attach to the specifed process: %s", err)
		}
	}

	if t.session != nil && t.session.IsDetached() {
		t.session, err = t.device.Attach(p.Pid(), frida_go.SessionOptions{})
		if err != nil {
			return errors.Errorf("Failed to attach to the specifed process: %s", err)
		}
	}

	scops := frida_go.ScriptOptions{
		Name:    "Script",
		Runtime: frida_go.FRIDA_SCRIPT_RUNTIME_QJS,
	}
	script, err := util.ReadTextFile(scriptFilePath)
	if err != nil {
		return errors.Errorf("Failed to read script file: %s", err)
	}

	t.script, err = t.session.CreateScript(script, scops)
	if err != nil {
		return errors.Errorf("Failed to create the script: %s", err)
	}

	t.script.OnMessage(func(sjson jsoniter.Any, data []byte) {
		//fmt.Println(sjson.ToString())
	})

	// t.script.OnDestroyed(func() {
	// 	fmt.Println("Script was destroyed")
	// })

	err = t.script.Load()
	if err != nil {
		return errors.Errorf("Failed to load the script: %s", err)
	}

	r, err := t.sendRpcCall("setTarget", handler)
	if err != nil || r != "true" {
		return errors.Errorf("Failed to set the coverage target: %s", err)
	}

	_, err = t.sendRpcCall("startCoverageFeed")
	if err != nil {
		t.Stop()
		return errors.Errorf("Failed to start the coverage feed: %s", err)
	}

	return nil
}

func (t *Trace) GetCoverage() ([]CoverageBlock, error) {
	r, err := t.sendRpcCall("getCoverage")
	if err != nil {
		return nil, errors.Errorf("Failed to fetch coverage information: %s", err)
	}
	unmarshalledCov := []RPCCoverage{}
	err = json.Unmarshal([]byte(r), &unmarshalledCov)
	if err != nil {
		return nil, errors.Errorf("Failed to unmarshal coverage information: %s", err)
	}

	if len(unmarshalledCov) == 0 {
		return nil, nil
	}

	covData := make([]CoverageBlock, 0, 5)
	for _, cov := range unmarshalledCov[0].Coverage {
		parsedStart, err := strconv.ParseUint(cov[0], 0, 64)
		if err != nil {
			continue
		}
		parsedEnd, err := strconv.ParseUint(cov[1], 0, 64)
		if err != nil {
			continue
		}
		covData = append(covData, CoverageBlock{
			Module:     unmarshalledCov[0].Module,
			BlockStart: parsedStart,
			BlockEnd:   parsedEnd,
		})
	}
	return covData, nil
}

func (t *Trace) ClearCoverage() error {
	_, err := t.sendRpcCall("clearCoverage")
	if err != nil {
		return errors.Errorf("Failed to clear coverage information: %s", err)
	}
	return nil
}

func (t *Trace) GetLastExecTime() (int, error) {
	res, err := t.sendRpcCall("getExecTime")
	if err != nil {
		return 0, errors.Errorf("Failed to clear coverage information: %s", err)
	}
	time, err := strconv.Atoi(res)
	if err != nil {
		return 0, err
	}
	return time, nil
}

func (t *Trace) Unload() error {
	if err := t.script.UnLoad(); err != nil {
		return errors.Errorf("Failed to unload the script: %s", err)
	}
	t.script.Free()
	return nil
}

func (t *Trace) Stop() error {
	if err := t.session.Detach(); err != nil {
		return errors.Errorf("Failed to detach the session: %s", err)
	}
	t.session.Free()
	//t.device.Free()

	// if err := t.manager.Close(); err != nil {
	// 	return errors.Errorf("Failed to close the manager: %s", err)
	// }
	return nil
}

func (t *Trace) sendRpcCall(methodName string, args ...interface{}) (string, error) {
	ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
	r, err := t.script.RpcCall(ctx, methodName, args)
	if err != nil {
		return "", errors.Errorf("Failed to call %s: %s ", methodName, err)
	}

	return r.ToString(), nil
}
