package main

import (
	"crypto/tls"
	"fmt"
	"log"

	"github.com/Natali-Skv/technopark_IS_http_proxy/config"
	"github.com/Natali-Skv/technopark_IS_http_proxy/internal/cert"
	proxyserver "github.com/Natali-Skv/technopark_IS_http_proxy/internal/proxyServer"

	// postgresTool "github.com/Natali-Skv/technopark_IS_http_proxy/internal/tools/postgresql"
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

	proxyServ := proxyserver.NewProxyServer(caCert, &tls.Config{MinVersion: tls.VersionTLS12}, nil)
	proxyServ.ListenAndServe(&servConf.Proxy)

}
