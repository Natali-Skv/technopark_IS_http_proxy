package main

import (
	"crypto/tls"
	"fmt"
	"log"

	"github.com/Natali-Skv/technopark_IS_http_proxy/config"
	"github.com/Natali-Skv/technopark_IS_http_proxy/internal/cert"
	proxyserver "github.com/Natali-Skv/technopark_IS_http_proxy/internal/proxyServer"
	"github.com/Natali-Skv/technopark_IS_http_proxy/internal/tools/logger/zaplogger"
	"github.com/Natali-Skv/technopark_IS_http_proxy/internal/tools/postgresql"
	"github.com/pkg/errors"

	// postgresTool "github.com/Natali-Skv/technopark_IS_http_proxy/internal/tools/postgresql"
	servLog "github.com/Natali-Skv/technopark_IS_http_proxy/internal/tools/logger"
	"github.com/spf13/viper"
)

func main() {
	viper.AddConfigPath("./config/")
	viper.SetConfigName("config")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal(err)
	}

	var servConf config.Config
	if err := viper.Unmarshal(&servConf); err != nil {
		log.Fatal(err)
	}
	fmt.Println(servConf)
	caCert, err := cert.LoadCA(servConf.Proxy.CaCrt, servConf.Proxy.CaKey, servConf.Proxy.CommonName)
	if err != nil {
		log.Fatal(err)
	}

	// postgresTool.DBConn(servConf.DB)

	logger, err := zaplogger.NewZapLogger(&servConf.Logger)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error creating logger object"))
	}
	defer func() {
		err := logger.Sync()
		if err != nil {
			log.Fatal("Error occurred in logger sync")
		}
	}()

	servLogger := servLog.NewServLogger(logger)

	pgxManager, err := postgresql.NewDBConn(&servConf.DB)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error creating postgres agent"))
	}
	defer pgxManager.Close()

	proxyRepo := proxyserver.NewProxyRepository(pgxManager)
	comonMw := proxyserver.NewCommonMiddleware(servLogger)

	proxyServ := proxyserver.NewProxyServer(proxyRepo, caCert, &tls.Config{MinVersion: tls.VersionTLS12}, nil)
	proxyServ.ListenAndServe(&servConf.Proxy, comonMw)

}
