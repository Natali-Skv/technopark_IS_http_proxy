package middleware

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/Natali-Skv/technopark_IS_http_proxy/internal/tools/logger"
	"github.com/labstack/echo/v4"
)

const (
	RequestIdCtxKey = "reqId"
	LoggerCtxKey    = "logger"
)

var requestId uint64 = 1

type CommonMiddleware struct {
	Logger *log.ServLogger
}

func NewCommonMiddleware(logger *log.ServLogger) *CommonMiddleware {
	return &CommonMiddleware{
		Logger: logger,
	}
}

func (mw *CommonMiddleware) RequestIdMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		requestId++
		currReqId := requestId
		ctx.Set(RequestIdCtxKey, currReqId)
		return next(ctx)
	}
}

func GetRequestIdFromCtx(ctx echo.Context) uint64 {
	reqId, ok := ctx.Get(RequestIdCtxKey).(uint64)
	if !ok {
		return 0
	}
	return reqId
}

func (mw *CommonMiddleware) PanicMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		defer func() {
			if err := recover(); err != nil {
				requestId := GetRequestIdFromCtx(ctx)
				mw.Logger.Error(requestId, "panic recovered: "+fmt.Sprint(err))
				mw.Logger.Access(requestId, ctx.Request().Method, ctx.Request().RemoteAddr, ctx.Request().Host, ctx.Request().URL.Path, time.Duration(0))
				_ = ctx.JSON(http.StatusInternalServerError, struct {
					Error string `json:"error"`
				}{Error: "internal server error"})
				if err != nil {
					return
				}
			}
		}()
		return next(ctx)
	}
}

func (mw *CommonMiddleware) AccessLogMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		reqId := GetRequestIdFromCtx(ctx)
		ctx.Set(LoggerCtxKey, mw.Logger)
		start := time.Now()
		result := next(ctx)
		mw.Logger.Access(reqId, ctx.Request().Method, ctx.Request().RemoteAddr, ctx.Request().Host, ctx.Request().URL.Path, time.Since(start))
		return result
	}
}

func GetLoggerFromCtx(ctx echo.Context) *log.ServLogger {
	logger, ok := ctx.Get(LoggerCtxKey).(*log.ServLogger)
	if !ok {
		return nil
	}
	return logger
}
