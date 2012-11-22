package logger

import (
	"log"
)

const (
	DEBUG = iota
	INFO
	ERROR
	FATAL
)

var LEVEL_PREFIXES = map[int]string{
	DEBUG: "[DEBUG] ",
	INFO:  "[INFO] ",
	ERROR: "[ERROR] ",
	FATAL: "[FATAL] ",
}

var LEVEL = FATAL

func Log(level int, format string, v ...interface{}) {
	if level != FATAL {
		log.Printf(LEVEL_PREFIXES[level]+format, v...)
	} else {
		log.Fatalf(LEVEL_PREFIXES[level]+format, v...)
	}
}

func LogDebug(format string, v ...interface{}) {
	Log(DEBUG, format, v...)
}

func LogInfo(format string, v ...interface{}) {
	Log(INFO, format, v...)
}

func LogError(format string, v ...interface{}) {
	Log(ERROR, format, v...)
}

func LogFatal(format string, v ...interface{}) {
	Log(FATAL, format, v...)
}
