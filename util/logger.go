package util

import (
	"log"
	"os"
)

type Logger interface {
	Initialize(fileName string)
	LogInfo(data string)
	LogWarning(data string)
	LogError(data string)
}

type Log struct {
	WarningLogger *log.Logger
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
}

func NewLogger(fileName string) *Log {
	logFile, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	return &Log{
		InfoLogger:    log.New(logFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		WarningLogger: log.New(logFile, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile),
		ErrorLogger:   log.New(logFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

func (l *Log) LogInfo(data string) {
	l.InfoLogger.Println(data)
}

func (l *Log) LogWarning(data string) {
	l.WarningLogger.Println(data)
}

func (l *Log) LogError(data string) {
	l.ErrorLogger.Println(data)
}
