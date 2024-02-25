// Package logging provides logger creation.
package logging

import (
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"strings"

	"github.com/dekarrin/jellog"
	"github.com/dekarrin/jelly"
)

// New creates a new logger of the given provider. If filename is blank, it will
// not log to disk, only stderr, and the stderr logger will be configured at
// trace level instead of info level.
func New(p jelly.LogProvider, filename string) (jelly.Logger, error) {
	var err error

	switch p {
	case jelly.NoLog:
		return nil, errors.New("log provider cannot be NoLog")
	case jelly.Jellog:
		var logOut *jellog.FileHandler
		if filename != "" {
			logOut, err = jellog.OpenFile(filename, nil)
			if err != nil {
				return nil, fmt.Errorf("open logfile: %q: %w", filename, err)
			}
		}
		j := jellog.New(jellog.Defaults[string]().WithComponent("jelly"))

		if filename != "" {
			j.AddHandler(jellog.LvTrace, logOut)
			j.AddHandler(jellog.LvInfo, jellog.NewStderrHandler(nil))
		} else {
			j.AddHandler(jellog.LvTrace, jellog.NewStderrHandler(nil))
		}

		return jellogLogger{j: j}, nil
	case jelly.StdLog:
		var logWriter io.Writer = os.Stderr
		if filename != "" {
			fileWriter, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				return nil, fmt.Errorf("open logfile: %q: %w", filename, err)
			}
			logWriter = io.MultiWriter(os.Stderr, fileWriter)
		}
		return stdLogger{std: stdlog.New(logWriter, "", stdlog.Ldate|stdlog.Ltime|stdlog.LUTC)}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %q", p.String())
	}
}

// NoOpLogger is a logger that performs no operations.
type NoOpLogger struct{}

func (log NoOpLogger) Debug(msg string)                            {}
func (log NoOpLogger) Warn(msg string)                             {}
func (log NoOpLogger) Trace(msg string)                            {}
func (log NoOpLogger) Info(msg string)                             {}
func (log NoOpLogger) Error(msg string)                            {}
func (log NoOpLogger) Debugf(msg string, a ...interface{})         {}
func (log NoOpLogger) Warnf(msg string, a ...interface{})          {}
func (log NoOpLogger) Tracef(msg string, a ...interface{})         {}
func (log NoOpLogger) Infof(msg string, a ...interface{})          {}
func (log NoOpLogger) Errorf(msg string, a ...interface{})         {}
func (log NoOpLogger) ErrorBreak()                                 {}
func (log NoOpLogger) InfoBreak()                                  {}
func (log NoOpLogger) WarnBreak()                                  {}
func (log NoOpLogger) TraceBreak()                                 {}
func (log NoOpLogger) DebugBreak()                                 {}
func (log NoOpLogger) LogResult(req *http.Request, r jelly.Result) {}

type stdLogger struct {
	std *stdlog.Logger
}

func (log stdLogger) Trace(msg string) {
	log.std.Print("TRACE " + msg)
}

func (log stdLogger) Tracef(msg string, a ...interface{}) {
	log.std.Printf("TRACE "+msg, a...)
}

func (log stdLogger) TraceBreak() {
	log.std.Printf("")
}

func (log stdLogger) Debug(msg string) {
	log.std.Print("DEBUG " + msg)
}

func (log stdLogger) Debugf(msg string, a ...interface{}) {
	log.std.Printf("DEBUG "+msg, a...)
}

func (log stdLogger) DebugBreak() {
	log.std.Printf("")
}

func (log stdLogger) Info(msg string) {
	log.std.Print("INFO  " + msg)
}

func (log stdLogger) Infof(msg string, a ...interface{}) {
	log.std.Printf("INFO  "+msg, a...)
}

func (log stdLogger) InfoBreak() {
	log.std.Printf("")
}

func (log stdLogger) Warn(msg string) {
	log.std.Print("WARN  " + msg)
}

func (log stdLogger) Warnf(msg string, a ...interface{}) {
	log.std.Printf("WARN  "+msg, a...)
}

func (log stdLogger) WarnBreak() {
	log.std.Printf("")
}

func (log stdLogger) Error(msg string) {
	log.std.Print("ERROR " + msg)
}

func (log stdLogger) Errorf(msg string, a ...interface{}) {
	log.std.Printf("DEBUG "+msg, a...)
}

func (log stdLogger) ErrorBreak() {
	log.std.Printf("")
}

func (log stdLogger) LogResult(req *http.Request, r jelly.Result) {
	logHTTPResponse(log, req, r)
}

type jellogLogger struct {
	j jellog.Logger[string]
}

func (log jellogLogger) Debug(msg string) {
	log.j.Debug(msg)
}

func (log jellogLogger) Debugf(msg string, a ...interface{}) {
	log.j.Debugf(msg, a...)
}

func (log jellogLogger) Warn(msg string) {
	log.j.Warn(msg)
}

func (log jellogLogger) Warnf(msg string, a ...interface{}) {
	log.j.Warnf(msg, a...)
}

func (log jellogLogger) Trace(msg string) {
	log.j.Trace(msg)
}

func (log jellogLogger) Tracef(msg string, a ...interface{}) {
	log.j.Tracef(msg, a...)
}

func (log jellogLogger) Info(msg string) {
	log.j.Info(msg)
}

func (log jellogLogger) Infof(msg string, a ...interface{}) {
	log.j.Infof(msg, a...)
}

func (log jellogLogger) Error(msg string) {
	log.j.Error(msg)
}

func (log jellogLogger) Errorf(msg string, a ...interface{}) {
	log.j.Errorf(msg, a...)
}

func (log jellogLogger) ErrorBreak() {
	log.j.InsertBreak(jellog.LvError)
}

func (log jellogLogger) InfoBreak() {
	log.j.InsertBreak(jellog.LvInfo)
}

func (log jellogLogger) WarnBreak() {
	log.j.InsertBreak(jellog.LvWarn)
}

func (log jellogLogger) TraceBreak() {
	log.j.InsertBreak(jellog.LvTrace)
}

func (log jellogLogger) DebugBreak() {
	log.j.InsertBreak(jellog.LvDebug)
}

func (log jellogLogger) LogResult(req *http.Request, r jelly.Result) {
	logHTTPResponse(log, req, r)
}

func logHTTPResponse(log jelly.Logger, req *http.Request, r jelly.Result) {
	// we don't really care about the ephemeral port from the client end
	remoteAddrParts := strings.SplitN(req.RemoteAddr, ":", 2)
	remoteIP := remoteAddrParts[0]

	if r.IsErr {
		log.Errorf("%s %s %s: HTTP-%d %s", remoteIP, req.Method, req.URL.Path, r.Status, r.InternalMsg)
	} else {
		log.Infof("%s %s %s: HTTP-%d %s", remoteIP, req.Method, req.URL.Path, r.Status, r.InternalMsg)
	}
}
