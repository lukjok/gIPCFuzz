package events

import (
	"fmt"
	"time"

	winlog "github.com/ofcoursedude/gowinlog"
)

func GetApplicationEventChannel() (*winlog.WinLogWatcher, error) {
	watcher, err := winlog.NewWinLogWatcher()
	if err != nil {
		fmt.Printf("Couldn't create watcher: %v\n", err)
		return nil, err
	}
	err = watcher.SubscribeFromNow("Application", "*")
	if err != nil {
		fmt.Printf("Couldn't subscribe to Application: %v", err)
		return nil, err
	}
	for {
		select {
		case evt := <-watcher.Event():
			// Print the event struct
			// fmt.Printf("\nEvent: %v\n", evt)
			// or print basic output
			fmt.Printf("\n%s: %s: %s\n", evt.LevelText, evt.ProviderName, evt.Msg)
		case err := <-watcher.Error():
			fmt.Printf("\nError: %v\n\n", err)
		default:
			// If no event is waiting, need to wait or do something else, otherwise
			// the the app fails on deadlock.
			<-time.After(1 * time.Millisecond)
		}
	}
}
