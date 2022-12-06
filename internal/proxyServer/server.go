package proxyserver

import (
	"bufio"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/Natali-Skv/technopark_IS_http_proxy/config"
	"github.com/Natali-Skv/technopark_IS_http_proxy/internal/cert"
	httperrors "github.com/Natali-Skv/technopark_IS_http_proxy/internal/utils/httpErrors"
	"github.com/Natali-Skv/technopark_IS_http_proxy/internal/utils/middleware"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	// "github.com/labstack/echo-contrib/pprof"
)

var okHeader = []byte("HTTP/1.1 200 OK\r\n\r\n")

type ProxyServer struct {
	repo ProxyRepository
	CA   *tls.Certificate
	// proxy server's tls-config for connecting to client as server
	ProxyAsServerTLSConfig *tls.Config

	// proxy server's tls-config for connecting to upstream-server as client
	ProxyAsClientTLSConfig *tls.Config
}

func NewProxyServer(repo *ProxyRepository, caCert *tls.Certificate, servConf, clientConf *tls.Config) *ProxyServer {
	return &ProxyServer{
		repo:                   *repo,
		CA:                     caCert,
		ProxyAsServerTLSConfig: servConf,
		ProxyAsClientTLSConfig: clientConf,
	}
}

func (ps *ProxyServer) ListenAndServe(proxyConf *config.ServerConfig, mw *middleware.CommonMiddleware) {
	e := echo.New()
	// pprof.Register(e)
	e.Use(echomw.Recover(), mw.RequestIdMiddleware, mw.AccessLogMiddleware, mw.PanicMiddleware, ps.proxyDefineProtocol)

	httpServ := http.Server{
		Addr:         proxyConf.Addr(),
		ReadTimeout:  time.Duration(proxyConf.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(proxyConf.WriteTimeout) * time.Second,
		Handler:      e,
	}

	e.Logger.Fatal(e.StartServer(&httpServ))
}

func (ps *ProxyServer) proxyDefineProtocol(_ echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		if ctx.Request().Method == http.MethodConnect {
			return ps.proxyHTTPSHandler(ctx)
		}
		return ps.proxyHTTPHandler(ctx)
	}
}

func (ps *ProxyServer) proxyHTTPHandler(ctx echo.Context) error {
	logger := middleware.GetLoggerFromCtx(ctx)
	requestId := middleware.GetRequestIdFromCtx(ctx)
	ctx.Request().Header.Del("Proxy-Connection")

	reqDump, err := httputil.DumpRequest(ctx.Request(), true)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "request dump error").Error())
		return echo.NewHTTPError(http.StatusServiceUnavailable, httperrors.INTERNAL_SERVER_ERR)
	}

	repoReq := FormRequestData(ctx.Request(), reqDump)
	repoReq.IsHTTPS = false
	repoReqID, err := ps.repo.InsertRequest(repoReq)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "http inserting request to db error").Error())
		return echo.NewHTTPError(http.StatusServiceUnavailable, httperrors.INTERNAL_SERVER_ERR)
	}

	upstreamResp, err := http.DefaultTransport.RoundTrip(ctx.Request())
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "round trip").Error())
		return echo.NewHTTPError(http.StatusServiceUnavailable, httperrors.INTERNAL_SERVER_ERR)
	}
	defer upstreamResp.Body.Close()

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
	var upsreamBody string
	if b, err := io.ReadAll(upstreamResp.Body); err == nil {
		upsreamBody = string(b)
	}

	upstreamRepoResp := FormResponseData(upstreamResp, upsreamBody)
	if upstreamResp == nil {
		logger.Error(requestId, errors.Wrap(err, "form response error").Error())
		return echo.NewHTTPError(http.StatusInternalServerError, httperrors.INTERNAL_SERVER_ERR)
	}

	err = ps.repo.InsertResponse(repoReqID, upstreamRepoResp)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "http inserting response to db error").Error())
		return echo.NewHTTPError(http.StatusInternalServerError, httperrors.INTERNAL_SERVER_ERR)
	}

	return nil
}

func (ps *ProxyServer) proxyHTTPSHandler(ctx echo.Context) error {
	logger := middleware.GetLoggerFromCtx(ctx)
	requestId := middleware.GetRequestIdFromCtx(ctx)
	name, _, _ := net.SplitHostPort(ctx.Request().Host)

	if name == "" {
		logger.Warn(requestId, "cannot determine cert name for"+ctx.Request().Host)
		return echo.NewHTTPError(http.StatusServiceUnavailable, httperrors.NO_UPSTREAM_ERR)
	}

	provisionalCert, err := cert.GenCert(ps.CA, name)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "generating leaf provisional cert").Error())
		return echo.NewHTTPError(http.StatusInternalServerError, httperrors.INTERNAL_SERVER_ERR)
	}

	serverConfig := tls.Config{}
	if ps.ProxyAsServerTLSConfig != nil {
		serverConfig = *ps.ProxyAsServerTLSConfig
	}
	serverConfig.Certificates = []tls.Certificate{*provisionalCert}
	var connToUpstream *tls.Conn
	serverConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		clientConfig := tls.Config{}
		if ps.ProxyAsClientTLSConfig != nil {
			clientConfig = *ps.ProxyAsClientTLSConfig
		}
		clientConfig.ServerName = hello.ServerName
		connToUpstream, err = tls.Dial("tcp", ctx.Request().Host, &clientConfig)
		if err != nil {
			logger.Error(requestId, errors.Wrap(err, "dial error").Error())
			return nil, err
		}
		return cert.GenCert(ps.CA, hello.ServerName)
	}

	hijackedConnToClient, _, err := ctx.Response().Hijack()
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "hijacking error:").Error())
		return nil
	}
	defer hijackedConnToClient.Close()

	if _, err = hijackedConnToClient.Write(okHeader); err != nil {
		logger.Error(requestId, errors.Wrap(err, "writing ok-header error").Error())
		return nil
	}

	connToClient := tls.Server(hijackedConnToClient, &serverConfig)
	if connToClient == nil {
		logger.Error(requestId, errors.Wrap(err, "tls-server error:").Error())
		return nil
	}
	defer connToClient.Close()

	err = connToClient.Handshake()
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "tls-server error:").Error())
		return nil
	}

	if connToUpstream == nil {
		logger.Warn(requestId, "connection to upstrean error")
		return nil
	}
	defer connToUpstream.Close()

	reader := bufio.NewReader(connToClient)
	request, err := http.ReadRequest(reader)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "getting request error").Error())
		return nil
	}

	requestByte, err := httputil.DumpRequest(request, true)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "dump request error").Error())
		return nil
	}

	repoReq := FormRequestData(request, requestByte)
	repoReq.IsHTTPS = true
	repoReqID, err := ps.repo.InsertRequest(repoReq)

	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "https inserting request to db error").Error())
		return nil
	}

	_, err = connToUpstream.Write(requestByte)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "write request error").Error())
		return nil
	}

	serverReader := bufio.NewReader(connToUpstream)
	response, err := http.ReadResponse(serverReader, request)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "read response error").Error())
		return nil
	}

	rawResponse, err := httputil.DumpResponse(response, true)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "dump response error").Error())
		return nil
	}

	_, err = connToClient.Write(rawResponse)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "write response error").Error())
		return nil
	}

	var upsreamRespBody string
	if b, err := io.ReadAll(response.Body); err == nil {
		upsreamRespBody = string(b)
	}
	upstreamRepoResp := FormResponseData(response, upsreamRespBody)
	if upstreamRepoResp == nil {
		logger.Error(requestId, errors.Wrap(err, "form response error").Error())
		return nil
	}

	err = ps.repo.InsertResponse(repoReqID, upstreamRepoResp)
	if err != nil {
		logger.Error(requestId, errors.Wrap(err, "https inserting response to db error").Error())
		return nil
	}

	return nil
}
