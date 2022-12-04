package config

type ServerConfig struct {
	Host         string
	Port         string
	ReadTimeout  int
	WriteTimeout int
	CaCrt        string
	CaKey        string
	CommonName   string
}

func (srv ServerConfig) Addr() string {
	return srv.Host + ":" + srv.Port
}

type DBConfig struct {
	Host           string
	Port           string
	Username       string
	Password       string
	DBName         string
	MaxConnections int
}

type Config struct {
	Proxy    ServerConfig
	Repeater ServerConfig
	DB       DBConfig
}
