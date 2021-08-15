package trace

import (
	"context"
	"fmt"
	"log"
	"time"

	frida_go "github.com/a97077088/frida-go"
	jsoniter "github.com/json-iterator/go"
	"github.com/lukjok/gipcfuzz/util"
	"github.com/pkg/errors"
)

const (
	scriptFilePath = "C:\\Users\\lukas\\Downloads\\frida-core\\script.js"
)

type TraceManager interface {
	Start(uint) error
	GetCoverage() error
	Stop() error
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

func (t *Trace) Start(pid uint) error {
	p, err := t.device.GetProcessById(pid, frida_go.ProcessMatchOptions{})
	if err != nil {
		return errors.Errorf("Failed to find specified process: %s", err)
	}

	t.session, err = t.device.Attach(p.Pid(), frida_go.SessionOptions{})
	if err != nil {
		return errors.Errorf("Failed to attach to the specifed process: %s", err)
	}

	scops := frida_go.ScriptOptions{
		Name:    "Test",
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
		fmt.Println(sjson.ToString())
	})

	t.script.OnDestroyed(func() {
		fmt.Println("Script was destroyed")
	})

	err = t.script.Load()
	if err != nil {
		return errors.Errorf("Failed to load the script: %s", err)
	}

	return nil
}

func (t *Trace) GetCoverage() error {
	ctx, _ := context.WithTimeout(context.TODO(), time.Second*10)
	r, err := t.script.RpcCall(ctx, "getCoverage")
	if err != nil {
		return errors.Errorf("Failed to fetch coverage information: %s", err)
	}
	fmt.Println(r.ToString())
	return nil
}

func (t *Trace) Stop() error {
	if err := t.script.UnLoad(); err != nil {
		return errors.Errorf("Failed to unload the script: %s", err)
	}
	t.script.Free()

	if err := t.session.Detach(); err != nil {
		return errors.Errorf("Failed to detach the session: %s", err)
	}
	t.session.Free()
	t.device.Free()

	if err := t.manager.Close(); err != nil {
		return errors.Errorf("Failed to close the manager: %s", err)
	}
	return nil
}
