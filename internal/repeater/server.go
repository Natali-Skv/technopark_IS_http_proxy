package repeater

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Natali-Skv/technopark_IS_http_proxy/config"
	httperrors "github.com/Natali-Skv/technopark_IS_http_proxy/internal/utils/httpErrors"
	"github.com/Natali-Skv/technopark_IS_http_proxy/internal/utils/middleware"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
)

type RepeaterServer struct {
	repo RepeaterRepository
	CA   *tls.Certificate
	// proxy server's tls-config for connecting to client as server
	ProxyAsServerTLSConfig *tls.Config

	// proxy server's tls-config for connecting to upstream-server as client
	ProxyAsClientTLSConfig *tls.Config
}

func NewRepeaterServer(repo *RepeaterRepository, caCert *tls.Certificate, servConf, clientConf *tls.Config) *RepeaterServer {
	return &RepeaterServer{
		repo:                   *repo,
		CA:                     caCert,
		ProxyAsServerTLSConfig: servConf,
		ProxyAsClientTLSConfig: clientConf,
	}
}

func (rs *RepeaterServer) ListenAndServe(repeaterConf *config.ServerConfig, mw *middleware.CommonMiddleware) {
	e := echo.New()
	e.Use(echomw.Recover(), mw.RequestIdMiddleware, mw.AccessLogMiddleware, mw.PanicMiddleware)

	httpServ := http.Server{
		Addr:         repeaterConf.Addr(),
		ReadTimeout:  time.Duration(repeaterConf.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(repeaterConf.WriteTimeout) * time.Second,
		Handler:      e,
	}

	e.GET("/requests", rs.HandleAllRequests)
	e.GET("/requests/:id", rs.HandleRequestByID)
	e.GET("/repeat/:id", rs.HandleRepeatRequest)

	e.Logger.Fatal(e.StartServer(&httpServ))
}

func (rs *RepeaterServer) HandleAllRequests(ctx echo.Context) error {
	logger := middleware.GetLoggerFromCtx(ctx)
	requestId := middleware.GetRequestIdFromCtx(ctx)
	requests, err := rs.repo.GetAllRequests()
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "request dump error").Error())
		return echo.NewHTTPError(http.StatusServiceUnavailable, httperrors.INTERNAL_SERVER_ERR)
	}
	return ctx.JSON(http.StatusOK, requests)
}

func (rs *RepeaterServer) HandleRepeatRequest(ctx echo.Context) error {
	logger := middleware.GetLoggerFromCtx(ctx)
	requestId := middleware.GetRequestIdFromCtx(ctx)

	reqId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || reqId < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, httperrors.BAD_REQUEST_ID)
	}
	req, err := rs.repo.GetRequestByID(reqId)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "GetRequestByID error").Error())
		return echo.NewHTTPError(http.StatusInternalServerError, httperrors.INTERNAL_SERVER_ERR)
	}
	if req == nil {
		return echo.NewHTTPError(http.StatusBadRequest, httperrors.NO_SUCH_REQUEST)
	}

	httpReq, err := http.ReadRequest(bufio.NewReader(strings.NewReader(req.Raw)))
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "http ReadRequest error").Error())
		return echo.NewHTTPError(http.StatusInternalServerError, httperrors.INTERNAL_SERVER_ERR)
	}
	var upstreamResp *http.Response

	host, ok := req.Headers["Host"].(string)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, httperrors.NO_UPSTREAM_ERR)
	}

	httpReq.Host = host
	httpReq.URL.Host = host
	httpReq.URL.Scheme = "http"
	httpReq.URL.Opaque = ""

	if req.IsHTTPS {
		httpReq.Host = fmt.Sprintf("%s:%s", host, "443")
		clientConfig := tls.Config{}
		if rs.ProxyAsClientTLSConfig != nil {
			clientConfig = *rs.ProxyAsClientTLSConfig
		}
		clientConfig.InsecureSkipVerify = true
		connToUpstream, err := tls.Dial("tcp", httpReq.Host, &clientConfig)
		if err != nil {
			logger.Error(requestId, errors.Wrap(err, "dial error").Error())
			return echo.NewHTTPError(http.StatusServiceUnavailable, httperrors.UPSTREAM_UNAVAIBLE_ERR)
		}
		defer connToUpstream.Close()

		_, err = connToUpstream.Write([]byte(req.Raw))
		if err != nil {
			logger.Error(requestId, errors.Wrap(err, "write request error").Error())
			return nil
		}

		serverReader := bufio.NewReader(connToUpstream)
		upstreamResp, err = http.ReadResponse(serverReader, httpReq)
		if err != nil {
			logger.Error(requestId, errors.Wrap(err, "read response error").Error())
			return nil
		}

	} else {
		upstreamResp, err = http.DefaultTransport.RoundTrip(httpReq)
		if err != nil {
			logger.Error(requestId, errors.Wrap(err, "round trip").Error())
			return echo.NewHTTPError(http.StatusServiceUnavailable, httperrors.INTERNAL_SERVER_ERR)
		}
		defer upstreamResp.Body.Close()

	}

	for key, values := range upstreamResp.Header {
		for _, value := range values {
			ctx.Response().Header().Add(key, value)
		}
	}

	ctx.Response().Status = upstreamResp.StatusCode
	if _, err = io.Copy(ctx.Response(), upstreamResp.Body); err != nil {
		logger.Error(requestId, errors.Wrap(err, "copy upstream's response to client").Error())
		return echo.NewHTTPError(http.StatusInternalServerError, httperrors.INTERNAL_SERVER_ERR)
	}

	return nil
}

func (rs *RepeaterServer) HandleRequestByID(ctx echo.Context) error {
	logger := middleware.GetLoggerFromCtx(ctx)
	requestId := middleware.GetRequestIdFromCtx(ctx)

	reqId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || reqId < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, httperrors.BAD_REQUEST_ID)
	}
	req, err := rs.repo.GetRequestByID(reqId)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "GetRequestByID error").Error())
		return echo.NewHTTPError(http.StatusInternalServerError, httperrors.INTERNAL_SERVER_ERR)
	}
	if req == nil {
		return echo.NewHTTPError(http.StatusBadRequest, httperrors.NO_SUCH_REQUEST)
	}
	return ctx.JSON(http.StatusOK, req)
}
