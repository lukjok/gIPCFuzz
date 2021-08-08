package events

import (
	"fmt"
	"log"
	"time"

	winlog "github.com/ofcoursedude/gowinlog"
)

const (
	DefaultWindowsQuery = "*[System[(Level=2) and (EventID=1000 or EventID=1026)]]"
)

type EventsManager interface {
	NewEventManager(string)
	StopCapture()
	StartCapture()
	GetEventData() []string
}

type Events struct {
	watcher *winlog.WinLogWatcher
	events  []winlog.WinLogEvent
}

func (e *Events) GetEventData() []string {
	data := make([]string, 0, 5)
	for _, event := range e.events {
		data = append(data, event.Msg)
	}
	return data
}

func (e *Events) StopCapture() {
	e.watcher.Shutdown()
}

func (e *Events) StartCapture() {
	go e.captureEvents()
}

func (e *Events) NewEventManager(searchQuery string) error {
	e.events = make([]winlog.WinLogEvent, 0, 5)

	var initError error
	e.watcher, initError = winlog.NewWinLogWatcher()
	if initError != nil {
		log.Printf("Couldn't create Windows event watcher: %v\n", initError)
		return initError
	}

	err := e.watcher.SubscribeFromNow("Application", searchQuery)
	if err != nil {
		log.Printf("Couldn't subscribe to Application with query %s: %v", searchQuery, err)
		return err
	}

	return nil
}

func (e *Events) captureEvents() {
	for {
		select {
		case evt := <-e.watcher.Event():
			e.events = append(e.events, *evt)
		case err := <-e.watcher.Error():
			fmt.Printf("Error: %v\n\n", err)
		default:
			// If no event is waiting, need to wait or do something else, otherwise
			// the the app fails on deadlock.
			<-time.After(1 * time.Millisecond)
		}
	}
}
