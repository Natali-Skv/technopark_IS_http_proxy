package logger

import (
	"time"
)

type Logger interface {
	Debugw(msg string, keysAndValues ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
	Fatalw(msg string, keysAndValues ...interface{})
	Infow(msg string, keysAndValues ...interface{})
	Panicw(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Sync() error
}

type ServLogger struct {
	Logger Logger
}

func NewServLogger(logger Logger) *ServLogger {
	return &ServLogger{
		Logger: logger,
	}
}

const (
	AccessMsg          = "access"
	ReqIdTitle         = "request_id"
	MethodTitle        = "method"
	RemoteAddrTitle    = "remote_addr"
	UrlTitle           = "url"
	ProcesingTimeTitle = "processing_time"
	ErrorMsgTitle      = "error_msg"
	HostTitle          = "host"
	WarningMsgTitle    = "warn_msg"
)

func (l ServLogger) Access(requestId uint64, method, remoteAddr, host, url string, procesingTime time.Duration) {
	l.Logger.Infow(
		AccessMsg,
		ReqIdTitle, requestId,
		MethodTitle, method,
		RemoteAddrTitle, remoteAddr,
		UrlTitle, url,
		HostTitle, host,
		ProcesingTimeTitle, procesingTime,
	)
}

func (l ServLogger) Error(reqId uint64, errorMsg string) {
	l.Logger.Errorw(
		"error",
		ReqIdTitle, reqId,
		ErrorMsgTitle, errorMsg,
	)
}

func (l ServLogger) Warn(reqId uint64, warningMsg string) {
	l.Logger.Warnw(
		"warn",
		ReqIdTitle, reqId,
		WarningMsgTitle, warningMsg,
	)
}
