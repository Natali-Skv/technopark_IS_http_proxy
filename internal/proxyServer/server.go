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
	"strings"
	"time"

	"github.com/Natali-Skv/technopark_IS_http_proxy/config"
	"github.com/Natali-Skv/technopark_IS_http_proxy/internal/cert"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	// "github.com/labstack/echo-contrib/pprof"
)

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
	var (
		err   error
		sconn *tls.Conn
		name  = dnsName(ctx.Request().Host)
	)

	if name == "" {
		log.Println("cannot determine cert name for " + ctx.Request().Host)
		return echo.NewHTTPError(http.StatusServiceUnavailable, "no upstream")
	}

	provisionalCert, err := ps.cert(name)
	if err != nil {
		log.Println("cert", err)
		return echo.NewHTTPError(http.StatusServiceUnavailable, "no upstream")
	}

	sConfig := new(tls.Config)
	if ps.TLSServerConfig != nil {
		*sConfig = *ps.TLSServerConfig
	}
	sConfig.Certificates = []tls.Certificate{*provisionalCert}
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
		return ps.cert(hello.ServerName)
	}

	cconn, err := handshake(ctx.Response().Writer, sConfig)
	if err != nil {
		log.Println("handshake", ctx.Request().Host, err)
		return echo.NewHTTPError(http.StatusServiceUnavailable, "handshake")
	}
	defer cconn.Close()

	if sconn == nil {
		log.Println("could not determine cert name for " + ctx.Request().Host)
		// TODO тут точно именно в этом ошибка?
		return echo.NewHTTPError(http.StatusServiceUnavailable, "could not determine cert name")
	}
	defer sconn.Close()

	// ================mine==========
	// fmt.Println("\n---NEW---")
	// fmt.Println(ctx.Request())
	// defer fmt.Println("---END---")

	// localConn, _, err := ctx.Response().Hijack()
	// if err != nil {
	// 	log.Printf("hijacking error: %v", err)
	// 	return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	// }
	// defer localConn.Close()

	// if _, err = localConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
	// 	log.Printf("Connection establishing failed: %v", err)
	// 	return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	// }

	// host := strings.Split(ctx.Request().Host, ":")[0]
	// tlsConfig, err := generateTLSConfig(ps.CA, host, ctx.Request().URL.Scheme)
	// if err != nil {
	// 	log.Printf("error getting cert: %v", err)
	// 	return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	// }

	// tlsLocalConn := tls.Server(localConn, tlsConfig)
	// defer tlsLocalConn.Close()
	// err = tlsLocalConn.Handshake()
	// if err != nil {
	// 	log.Printf("handshaking failed: %v", err)
	// 	return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	// }

	// remoteConn, err := tls.Dial("tcp", ctx.Request().URL.Host, tlsConfig)
	// if err != nil {
	// 	log.Println(err)
	// 	return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	// }
	// defer remoteConn.Close()

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

	request.URL.Scheme = "https"
	hostAndPort := strings.Split(ctx.Request().URL.Host, ":")
	request.URL.Host = hostAndPort[0]
	return nil
}

func dnsName(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return host
}

func (ps *ProxyServer) cert(names ...string) (*tls.Certificate, error) {
	return cert.GenCert(ps.CA, names)
}

var okHeader = []byte("HTTP/1.1 200 OK\r\n\r\n")

// handshake hijacks w's underlying net.Conn, responds to the CONNECT request
// and manually performs the TLS handshake. It returns the net.Conn or and
// error if any.
func handshake(w http.ResponseWriter, config *tls.Config) (net.Conn, error) {
	raw, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, "no upstream", 503)
		return nil, err
	}
	if _, err = raw.Write(okHeader); err != nil {
		raw.Close()
		return nil, err
	}
	conn := tls.Server(raw, config)
	err = conn.Handshake()
	if err != nil {
		conn.Close()
		raw.Close()
		return nil, err
	}
	return conn, nil
}
