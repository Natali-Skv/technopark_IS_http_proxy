package proxyserver

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/Natali-Skv/technopark_IS_http_proxy/config"
	"github.com/Natali-Skv/technopark_IS_http_proxy/internal/cert"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	// "github.com/labstack/echo-contrib/pprof"
)

var okHeader = []byte("HTTP/1.1 200 OK\r\n\r\n")

type ProxyServer struct {
	// bd
	// CA specifies the root CA for generating leaf certs for each incoming
	// TLS request.
	CA *tls.Certificate

	// TLSServerConfig specifies the tls.Config to use when generating leaf
	// cert using CA.
	TLSServerConfig *tls.Config

	// TLSClientConfig specifies the tls.Config to use when establishing
	// an upstream connection for proxying.
	TLSClientConfig *tls.Config
}

func NewProxyServer(caCert *tls.Certificate, servConf, clientConf *tls.Config) *ProxyServer {
	return &ProxyServer{
		CA:              caCert,
		TLSServerConfig: servConf,
		TLSClientConfig: clientConf,
	}
}

func (ps *ProxyServer) ListenAndServe(proxyConf *config.ServerConfig) {
	e := echo.New()
	// pprof.Register(e)
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	e.Use(ps.proxyDefineProtocol)

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
			fmt.Println("HTTPS_HTTPS_HTTPS")
			return ps.proxyHTTPSHandler(ctx)
		}
		fmt.Println("HTTP_HTTP")
		return ps.proxyHTTPHandler(ctx)
	}
}

func (ps *ProxyServer) proxyHTTPHandler(ctx echo.Context) error {
	defer fmt.Println("---END---\n\n")

	ctx.Request().Header.Del("Proxy-Connection")

	resp, err := http.DefaultTransport.RoundTrip(ctx.Request())
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			ctx.Response().Header().Add(key, value)
		}
	}

	ctx.Response().Status = resp.StatusCode
	if _, err = io.Copy(ctx.Response(), resp.Body); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	fmt.Println(*resp)

	return nil
}

func (ps *ProxyServer) proxyHTTPSHandler(ctx echo.Context) error {
	name, _, _ := net.SplitHostPort(ctx.Request().Host)

	if name == "" {
		log.Println("cannot determine cert name for " + ctx.Request().Host)
		return echo.NewHTTPError(http.StatusServiceUnavailable, "no upstream")
	}

	provisionalCert, err := cert.GenCert(ps.CA, name)
	if err != nil {
		log.Println("cert", err)
		return echo.NewHTTPError(http.StatusServiceUnavailable, "no upstream")
	}

	sConfig := new(tls.Config)
	if ps.TLSServerConfig != nil {
		*sConfig = *ps.TLSServerConfig
	}
	sConfig.Certificates = []tls.Certificate{*provisionalCert}
	var sconn *tls.Conn
	sConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		cConfig := new(tls.Config)
		if ps.TLSClientConfig != nil {
			*cConfig = *ps.TLSClientConfig
		}
		cConfig.ServerName = hello.ServerName
		sconn, err = tls.Dial("tcp", ctx.Request().Host, cConfig)
		if err != nil {
			log.Println("dial", ctx.Request().Host, err)
			return nil, err
		}
		return cert.GenCert(ps.CA, hello.ServerName)
	}

	hijackedConn, _, err := ctx.Response().Hijack()
	if err != nil {
		log.Printf("hijacking error: %v", err)
		return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	}
	defer hijackedConn.Close()

	if _, err = hijackedConn.Write(okHeader); err != nil {
		log.Printf("writing ok-header error: %v", err)
		return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	}

	cconn := tls.Server(hijackedConn, sConfig)
	if cconn == nil {
		log.Printf("tls-server error: %v", err)
		return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	}
	defer cconn.Close()

	err = cconn.Handshake()
	if err != nil {
		log.Println("handshake", ctx.Request().Host, err)
		return echo.NewHTTPError(http.StatusServiceUnavailable, "handshake")
	}

	if sconn == nil {
		log.Println("could not determine cert name for " + ctx.Request().Host)
		// TODO тут точно именно в этом ошибка?
		return echo.NewHTTPError(http.StatusServiceUnavailable, "could not determine cert name")
	}
	defer sconn.Close()

	reader := bufio.NewReader(cconn)
	request, err := http.ReadRequest(reader)
	if err != nil {
		log.Printf("error getting request: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	requestByte, err := httputil.DumpRequest(request, true)
	fmt.Println(string(requestByte))
	if err != nil {
		log.Printf("failed to dump request: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	_, err = sconn.Write(requestByte)
	if err != nil {
		log.Printf("failed to write request: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	serverReader := bufio.NewReader(sconn)
	response, err := http.ReadResponse(serverReader, request)
	if err != nil {
		log.Printf("failed to read response: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	rawResponse, err := httputil.DumpResponse(response, true)
	if err != nil {
		log.Printf("failed to dump response: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	_, err = cconn.Write(rawResponse)
	if err != nil {
		log.Printf("fail to write response: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return nil
}
