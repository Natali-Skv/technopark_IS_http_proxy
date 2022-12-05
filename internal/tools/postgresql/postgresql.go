package postgresql

import (
	"fmt"
	"log"

	config "github.com/Natali-Skv/technopark_IS_http_proxy/config"
	"github.com/jackc/pgx"
)

func NewDBConn(dbConf *config.DBConfig) (*pgx.ConnPool, error) {
	ConnStr := fmt.Sprintf("user=%s dbname=%s password=%s host=%s port=%s sslmode=disable",
		dbConf.Username,
		dbConf.DBName,
		dbConf.Password,
		dbConf.Host,
		dbConf.Port)

	pgxConnectionConfig, err := pgx.ParseConnectionString(ConnStr)
	if err != nil {
		log.Fatalf("Invalid config string: %s", err)
	}

	pool, err := pgx.NewConnPool(pgx.ConnPoolConfig{
		ConnConfig:     pgxConnectionConfig,
		MaxConnections: dbConf.MaxConnections,
		AfterConnect:   nil,
		AcquireTimeout: 0,
	})
	if err != nil {
		log.Fatalf("Error %s occurred during connection to database", err)
	}

	return pool, nil
}
