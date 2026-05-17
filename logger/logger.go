package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/op/go-logging"
)

var (
	logger      *logging.Logger
	logBufferMu sync.RWMutex
	logBuffer   = newLogRingBuffer(logBufferCapacity)
)

const logBufferCapacity = 10240

type bufferedLog struct {
	time   string
	level  logging.Level
	source string
	log    string
}

type logRingBuffer struct {
	items []bufferedLog
	next  int
	full  bool
}

func newLogRingBuffer(capacity int) *logRingBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &logRingBuffer{
		items: make([]bufferedLog, 0, capacity),
	}
}

func InitLogger(level logging.Level) {
	newLogger := logging.MustGetLogger("s-ui")
	var err error
	var backend logging.Backend
	var format logging.Formatter

	_, inContainer := os.LookupEnv("container")
	if !inContainer {
		if _, statErr := os.Stat("/.dockerenv"); statErr == nil {
			inContainer = true
		}
	}
	if inContainer {
		backend = logging.NewLogBackend(os.Stderr, "", 0)
		format = logging.MustStringFormatter(`%{time:2006/01/02 15:04:05} %{level} - %{message}`)
	} else {
		backend, err = logging.NewSyslogBackend("")
		if err != nil {
			fmt.Println("Unable to use syslog: " + err.Error())
			backend = logging.NewLogBackend(os.Stderr, "", 0)
		}
		if err != nil {
			format = logging.MustStringFormatter(`%{time:2006/01/02 15:04:05} %{level} - %{message}`)
		} else {
			format = logging.MustStringFormatter(`%{level} - %{message}`)
		}
	}

	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(level, "s-ui")
	newLogger.SetBackend(backendLeveled)

	logger = newLogger
}

func GetLogger() *logging.Logger {
	return logger
}

func Debug(args ...interface{}) {
	if logger == nil {
		fmt.Println(append([]interface{}{"DEBUG -"}, args...)...)
		return
	}
	logger.Debug(args...)
	addToBuffer("panel", "DEBUG", fmt.Sprint(args...))
}

func Debugf(format string, args ...interface{}) {
	if logger == nil {
		fmt.Printf("DEBUG - "+format+"\n", args...)
		return
	}
	logger.Debugf(format, args...)
	addToBuffer("panel", "DEBUG", fmt.Sprintf(format, args...))
}

func Info(args ...interface{}) {
	if logger == nil {
		fmt.Println(append([]interface{}{"INFO -"}, args...)...)
		return
	}
	logger.Info(args...)
	addToBuffer("panel", "INFO", fmt.Sprint(args...))
}

func Infof(format string, args ...interface{}) {
	if logger == nil {
		fmt.Printf("INFO - "+format+"\n", args...)
		return
	}
	logger.Infof(format, args...)
	addToBuffer("panel", "INFO", fmt.Sprintf(format, args...))
}

func Warning(args ...interface{}) {
	if logger == nil {
		fmt.Println(append([]interface{}{"WARNING -"}, args...)...)
		return
	}
	logger.Warning(args...)
	addToBuffer("panel", "WARNING", fmt.Sprint(args...))
}

func Warningf(format string, args ...interface{}) {
	if logger == nil {
		fmt.Printf("WARNING - "+format+"\n", args...)
		return
	}
	logger.Warningf(format, args...)
	addToBuffer("panel", "WARNING", fmt.Sprintf(format, args...))
}

func Error(args ...interface{}) {
	if logger == nil {
		fmt.Println(append([]interface{}{"ERROR -"}, args...)...)
		return
	}
	logger.Error(args...)
	addToBuffer("panel", "ERROR", fmt.Sprint(args...))
}

func Errorf(format string, args ...interface{}) {
	if logger == nil {
		fmt.Printf("ERROR - "+format+"\n", args...)
		return
	}
	logger.Errorf(format, args...)
	addToBuffer("panel", "ERROR", fmt.Sprintf(format, args...))
}

func CoreDebug(args ...interface{}) {
	logCore("DEBUG", fmt.Sprint(args...))
}

func CoreInfo(args ...interface{}) {
	logCore("INFO", fmt.Sprint(args...))
}

func CoreWarning(args ...interface{}) {
	logCore("WARNING", fmt.Sprint(args...))
}

func CoreError(args ...interface{}) {
	logCore("ERROR", fmt.Sprint(args...))
}

func logCore(level string, message string) {
	if logger == nil {
		fmt.Println(level+" -", message)
		return
	}
	switch level {
	case "DEBUG":
		logger.Debug(message)
	case "INFO":
		logger.Info(message)
	case "WARNING":
		logger.Warning(message)
	case "ERROR":
		logger.Error(message)
	}
	addToBuffer("core", level, message)
}

func addToBuffer(source string, level string, newLog string) {
	t := time.Now()
	logBufferMu.Lock()
	defer logBufferMu.Unlock()

	logLevel, _ := logging.LogLevel(level)
	logBuffer.append(bufferedLog{
		time:   t.Format("2006/01/02 15:04:05"),
		level:  logLevel,
		source: source,
		log:    newLog,
	})
}

func GetLogs(c int, level string) []string {
	return GetLogsFiltered(c, level, "", "")
}

func GetLogsFiltered(c int, level string, source string, filter string) []string {
	var output []string
	logLevel, _ := logging.LogLevel(level)

	logBufferMu.RLock()
	snapshot := logBuffer.snapshot()
	logBufferMu.RUnlock()

	for i := len(snapshot) - 1; i >= 0 && len(output) < c; i-- {
		entry := snapshot[i]
		if source != "" && entry.source != source {
			continue
		}
		if filter != "" && !strings.Contains(entry.log, filter) {
			continue
		}
		if entry.level <= logLevel {
			output = append(output, fmt.Sprintf("%s %s - %s", entry.time, entry.level, entry.log))
		}
	}
	return output
}

func (r *logRingBuffer) append(entry bufferedLog) {
	if cap(r.items) == 0 {
		r.items = make([]bufferedLog, 0, 1)
	}
	if len(r.items) < cap(r.items) {
		r.items = append(r.items, entry)
		if len(r.items) == cap(r.items) {
			r.full = true
			r.next = 0
		}
		return
	}
	r.items[r.next] = entry
	r.next = (r.next + 1) % len(r.items)
	r.full = true
}

func (r *logRingBuffer) snapshot() []bufferedLog {
	if len(r.items) == 0 {
		return nil
	}
	out := make([]bufferedLog, 0, len(r.items))
	if !r.full {
		return append(out, r.items...)
	}
	out = append(out, r.items[r.next:]...)
	out = append(out, r.items[:r.next]...)
	return out
}
