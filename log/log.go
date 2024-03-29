/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Austin Zhai
 * All rights reserved.
 */
package log

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
)

var (
	DefaultLog *Log

	ErrUnsupportedLogLevel = errors.New("unsupported log level")
)

type Level int

const (
	LevelNull  Level = 0
	LevelTrace Level = 1
	LevelDebug Level = 2
	LevelInfo  Level = 3
	LevelWarn  Level = 4
	LevelError Level = 5
	LevelFatal Level = 6

	traceS = "TRACE"
	debugS = "DEBUG"
	infoS  = "INFO"
	warnS  = "WARN"
	errorS = "ERROR"
	fatalS = "FATAL"
)

// These flags define which text to prefix to each log entry generated by the Logger.
// Bits are or'ed together to control what's printed.
// With the exception of the Lmsgprefix flag, there is no
// control over the order they appear (the order listed here)
// or the format they present (as described in the comments).
// The prefix is followed by a colon only when Llongfile or Lshortfile
// is specified.
// For example, flags Ldate | Ltime (or LstdFlags) produce,
//
//	2009/01/23 01:23:23 message
//
// while flags Ldate | Ltime | Lmicroseconds | Llongfile produce,
//
//	2009/01/23 01:23:23.123123 /a/b/c/d.go:23: message
const (
	Ldate         = 1 << iota // the date in the local time zone: 2009/01/23
	Ltime                     // the time in the local time zone: 01:23:23
	Lmicroseconds             // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                 // full file name and line number: /a/b/c/d.go:23
	Lshortfile                // final file name element and line number: d.go:23. overrides Llongfile
	LUTC                      // if Ldate or Ltime is set, use UTC rather than the local time zone
	Lmsgprefix                // move the "prefix" from the beginning of the line to before the message
	Lfuncname
	LstdFlags = Ldate | Ltime // initial values for the standard logger
)

var (
	levelStrings = map[Level]string{
		LevelTrace: traceS,
		LevelDebug: debugS,
		LevelInfo:  infoS,
		LevelWarn:  warnS,
		LevelError: errorS,
		LevelFatal: fatalS,
	}

	levelInts = map[string]Level{
		traceS: LevelTrace,
		debugS: LevelDebug,
		infoS:  LevelInfo,
		warnS:  LevelWarn,
		errorS: LevelError,
		fatalS: LevelFatal,
	}
)

func ParseLevel(levelS string) (Level, error) {
	level, ok := levelInts[strings.ToUpper(levelS)]
	if !ok {
		return LevelNull, ErrUnsupportedLogLevel
	}
	return level, nil
}

type Log struct {
	flag   int
	logger *log.Logger
	level  Level
	mu     sync.RWMutex
}

func init() {
	DefaultLog = NewLog()
}

func WithLevel(level Level) *Log {
	DefaultLog.mu.Lock()
	defer DefaultLog.mu.Unlock()
	DefaultLog.level = level
	return DefaultLog
}

func WithOutput(out io.Writer) *Log {
	DefaultLog.logger.SetOutput(out)
	return DefaultLog
}

func WithFlags(flag int) *Log {
	DefaultLog.logger.SetFlags(flag)
	DefaultLog.flag = flag
	return DefaultLog
}

func SetOutput(out io.Writer) {
	DefaultLog.logger.SetOutput(out)
}

func SetLevel(level Level) {
	DefaultLog.mu.Lock()
	defer DefaultLog.mu.Unlock()
	DefaultLog.level = level
}

func SetFlags(flag int) {
	DefaultLog.logger.SetFlags(flag)
	DefaultLog.flag = flag
}

func SetPrefix(prefix string) {
	DefaultLog.logger.SetPrefix(prefix)
}

func Println(level Level, v ...interface{}) {
	prefix, _ := levelStrings[level]
	prefix = fmt.Sprintf("%-6s", prefix)
	DefaultLog.outputln(level, prefix, v...)
}

func Printf(level Level, format string, v ...interface{}) {
	prefix, _ := levelStrings[level]
	prefix = fmt.Sprintf("%-6s", prefix)
	DefaultLog.outputf(false, level, prefix, format, v...)
}

func Trace(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", traceS)
	DefaultLog.outputln(LevelTrace, prefix, v...)
}

func Tracef(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", traceS)
	DefaultLog.outputf(true, LevelTrace, prefix, format, v...)
}

func Debug(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", debugS)
	DefaultLog.outputln(LevelDebug, prefix, v...)
}

func Debugf(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", debugS)
	DefaultLog.outputf(true, LevelDebug, prefix, format, v...)
}

func Info(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", infoS)
	DefaultLog.outputln(LevelInfo, prefix, v...)
}

func Infof(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", infoS)
	DefaultLog.outputf(true, LevelInfo, prefix, format, v...)
}

func Warn(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", warnS)
	DefaultLog.outputln(LevelWarn, prefix, v...)
}

func Warnf(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", warnS)
	DefaultLog.outputf(true, LevelWarn, prefix, format, v...)
}

func Error(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", errorS)
	DefaultLog.outputln(LevelError, prefix, v...)
}

func Errorf(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", errorS)
	DefaultLog.outputf(true, LevelError, prefix, format, v...)
}

func Fatal(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", fatalS)
	DefaultLog.outputln(LevelFatal, prefix, v...)
}

func Fatalf(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", fatalS)
	DefaultLog.outputf(true, LevelFatal, prefix, format, v...)
}

func NewLog() *Log {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	return &Log{
		logger: logger,
		level:  LevelInfo,
	}
}

func (log *Log) WithLevel(level Level) *Log {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.level = level
	return log
}

func (log *Log) WithOutput(out io.Writer) *Log {
	log.logger.SetOutput(out)
	return log
}

func (log *Log) WithFlags(flag int) *Log {
	log.logger.SetFlags(flag)
	return log
}

func (log *Log) SetOutput(out io.Writer) {
	log.logger.SetOutput(out)
	return
}

func (log *Log) SetLevel(level Level) {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.level = level
	return
}

func (log *Log) SetFlags(flag int) {
	log.logger.SetFlags(flag)
	return
}

func (log *Log) SetPrefix(prefix string) {
	log.logger.SetPrefix(prefix)
}

func (log *Log) Println(level Level, v ...interface{}) {
	prefix, _ := levelStrings[level]
	prefix = fmt.Sprintf("%-6s", prefix)
	log.outputln(level, prefix, v...)
}

func (log *Log) Printf(level Level, format string, v ...interface{}) {
	prefix, _ := levelStrings[level]
	prefix = fmt.Sprintf("%-6s", prefix)
	log.outputf(false, level, prefix, format, v...)
}

func (log *Log) Trace(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", traceS)
	log.outputln(LevelTrace, prefix, v...)
}

func (log *Log) Tracef(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", traceS)
	log.outputf(true, LevelTrace, prefix, format, v...)
}

func (log *Log) Debug(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", debugS)
	log.outputln(LevelDebug, prefix, v...)
}

func (log *Log) Debugf(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", debugS)
	log.outputf(true, LevelDebug, prefix, format, v...)
}

func (log *Log) Info(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", infoS)
	log.outputln(LevelInfo, prefix, v...)
}

func (log *Log) Infof(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", infoS)
	log.outputf(true, LevelInfo, prefix, format, v...)
}

func (log *Log) Warn(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", warnS)
	log.outputln(LevelWarn, prefix, v...)
}

func (log *Log) Warnf(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", warnS)
	log.outputf(true, LevelWarn, prefix, format, v...)
}

func (log *Log) Error(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", errorS)
	log.outputln(LevelError, prefix, v...)
}

func (log *Log) Errorf(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", errorS)
	log.outputf(true, LevelError, prefix, format, v...)
}

func (log *Log) Fatal(v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", fatalS)
	log.outputln(LevelFatal, prefix, v...)
}

func (log *Log) Fatalf(format string, v ...interface{}) {
	prefix := fmt.Sprintf("%-6s", fatalS)
	log.outputf(true, LevelFatal, prefix, format, v...)
}

func (log *Log) outputln(level Level, prefix string, v ...interface{}) {
	log.mu.RLock()
	defer log.mu.RUnlock()
	if level < log.level {
		return
	}
	var fun string
	if (log.flag | Lfuncname) != 0 {
		pc, _, _, ok := runtime.Caller(2)
		if ok {
			function := runtime.FuncForPC(pc)
			fun = function.Name()
		}
	}

	line := fmt.Sprintln(v...)
	line = prefix + fun + " " + line
	log.logger.Output(2, line)
	if level == LevelFatal {
		os.Exit(1)
	}
}

func (log *Log) outputf(newline bool, level Level, prefix, format string, v ...interface{}) {
	log.mu.RLock()
	defer log.mu.RUnlock()
	if level < log.level {
		return
	}
	var fun string
	if (log.flag | Lfuncname) != 0 {
		pc, _, _, ok := runtime.Caller(2)
		if ok {
			function := runtime.FuncForPC(pc)
			fun = function.Name()
		}
	}

	line := fmt.Sprintf(format, v...)
	line = prefix + fun + " " + line
	if newline {
		line += "\n"
	}
	log.logger.Output(2, line)
	if level == LevelFatal {
		os.Exit(1)
	}
}
