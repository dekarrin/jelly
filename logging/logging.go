package logging

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dekarrin/jellog"
	"github.com/dekarrin/jelly/types"
)

type Provider int

const (
	None Provider = iota
	Jellog
)

func (p Provider) String() string {
	switch p {
	case None:
		return "none"
	case Jellog:
		return "jellog"
	default:
		return fmt.Sprintf("Provider(%d)", int(p))
	}
}

func ParseProvider(s string) (Provider, error) {
	switch strings.ToLower(s) {
	case None.String(), "":
		return None, nil
	case Jellog.String():
		return Jellog, nil
	default:
		return None, fmt.Errorf("unknown Provider %q", s)
	}
}

// Logger is an object that is used to log messages. Use the New functions in
// the logging sub-package to create one.
type Logger interface {
	// Debug writes a message to the log at Debug level.
	Debug(string)

	// Debugf writes a formatted message to the log at Debug level.
	Debugf(string, ...interface{})

	// Error writes a message to the log at Error level.
	Error(string)

	// Errorf writes a formatted message to the log at Error level.
	Errorf(string, ...interface{})

	// Info writes a message to the log at Info level.
	Info(string)

	// Infof writes a formatted message to the log at Info level.
	Infof(string, ...interface{})

	// Trace writes a message to the log at Trace level.
	Trace(string)

	// Tracef writes a formatted message to the log at Trace level.
	Tracef(string, ...interface{})

	// Warn writes a message to the log at Warn level.
	Warn(string)

	// Warnf writes a formatted message to the log at Warn level.
	Warnf(string, ...interface{})

	// DebugBreak adds a 'break' between events in the log at Debug level. The
	// meaning of a break varies based on the underlying log; for text-based
	// logs, it is generally a newline character.
	DebugBreak()

	// ErrorBreak adds a 'break' between events in the log at Error level. The
	// meaning of a break varies based on the underlying log; for text-based
	// logs, it is generally a newline character.
	ErrorBreak()

	// InfoBreak adds a 'break' between events in the log at Info level. The
	// meaning of a break varies based on the underlying log; for text-based
	// logs, it is generally a newline character.
	InfoBreak()

	// TraceBreak adds a 'break' between events in the log at Trace level. The
	// meaning of a break varies based on the underlying log; for text-based
	// logs, it is generally a newline character.
	TraceBreak()

	// WarnBreak adds a 'break' between events in the log at Warn level. The
	// meaning of a break varies based on the underlying log; for text-based
	// logs, it is generally a newline character.
	WarnBreak()

	// LogResult logs a request and the response to that request.
	LogResult(req *http.Request, r types.Result)
}

// New creates a new logger of the given provider. If filename is blank, it will
// not log to disk, only stderr, and the stderr logger will be configured at
// trace level instead of info level.
func New(p Provider, filename string) (Logger, error) {
	var err error

	switch p {
	case None:
		return nil, fmt.Errorf("Logger provider cannot be None")
	case Jellog:
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
func (log NoOpLogger) LogResult(req *http.Request, r types.Result) {}

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

func (log jellogLogger) LogResult(req *http.Request, r types.Result) {
	if r.IsErr {
		log.logHTTPResponse("ERROR", req, r.Status, r.InternalMsg)
	} else {
		log.logHTTPResponse("INFO", req, r.Status, r.InternalMsg)
	}
}

func (log jellogLogger) logHTTPResponse(level string, req *http.Request, respStatus int, msg string) {
	// we don't really care about the ephemeral port from the client end
	remoteAddrParts := strings.SplitN(req.RemoteAddr, ":", 2)
	remoteIP := remoteAddrParts[0]

	if level == "ERROR" {
		log.Errorf("%s %s %s: HTTP-%d %s", remoteIP, req.Method, req.URL.Path, respStatus, msg)
	} else {
		log.Infof("%s %s %s: HTTP-%d %s", remoteIP, req.Method, req.URL.Path, respStatus, msg)
	}

	// original "log" provider style.
	// log.Printf("%s %s %s %s: HTTP-%d %s", level, remoteIP, req.Method, req.URL.Path, respStatus, msg)
}
