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

type LogConfig struct {
	Level            string
	Encoding         string
	OutputPaths      []string
	ErrorOutputPaths []string

	MessageKey    string
	TimeKey       string
	LevelKey      string
	NameKey       string
	FunctionKey   string
	StacktraceKey string
}

type Config struct {
	Proxy    ServerConfig
	Repeater ServerConfig
	DB       DBConfig
	Logger   LogConfig
}
