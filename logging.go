package jelly

// // Logger is an object that is used to log messages. Use the New functions in
// // the logging sub-package to create one.
// type Logger interface {
// 	// Debug writes a message to the log at Debug level.
// 	Debug(string)

// 	// Debugf writes a formatted message to the log at Debug level.
// 	Debugf(string, ...interface{})

// 	// Error writes a message to the log at Error level.
// 	Error(string)

// 	// Errorf writes a formatted message to the log at Error level.
// 	Errorf(string, ...interface{})

// 	// Info writes a message to the log at Info level.
// 	Info(string)

// 	// Infof writes a formatted message to the log at Info level.
// 	Infof(string, ...interface{})

// 	// Trace writes a message to the log at Trace level.
// 	Trace(string)

// 	// Tracef writes a formatted message to the log at Trace level.
// 	Tracef(string, ...interface{})

// 	// Warn writes a message to the log at Warn level.
// 	Warn(string)

// 	// Warnf writes a formatted message to the log at Warn level.
// 	Warnf(string, ...interface{})

// 	// DebugBreak adds a 'break' between events in the log at Debug level. The
// 	// meaning of a break varies based on the underlying log; for text-based
// 	// logs, it is generally a newline character.
// 	DebugBreak()

// 	// ErrorBreak adds a 'break' between events in the log at Error level. The
// 	// meaning of a break varies based on the underlying log; for text-based
// 	// logs, it is generally a newline character.
// 	ErrorBreak()

// 	// InfoBreak adds a 'break' between events in the log at Info level. The
// 	// meaning of a break varies based on the underlying log; for text-based
// 	// logs, it is generally a newline character.
// 	InfoBreak()

// 	// TraceBreak adds a 'break' between events in the log at Trace level. The
// 	// meaning of a break varies based on the underlying log; for text-based
// 	// logs, it is generally a newline character.
// 	TraceBreak()

// 	// WarnBreak adds a 'break' between events in the log at Warn level. The
// 	// meaning of a break varies based on the underlying log; for text-based
// 	// logs, it is generally a newline character.
// 	WarnBreak()

// 	// LogResult logs a request and the response to that request.
// 	LogResult(req *http.Request, r response.Result)
// }
