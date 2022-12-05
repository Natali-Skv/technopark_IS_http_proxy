package proxyserver

import (
	"github.com/jackc/pgx"
	"github.com/pkg/errors"
)

type ProxyRepository struct {
	conn *pgx.ConnPool
}

const (
	insertRequestQuery  = `INSERT INTO requests(method, path, get_params, headers, cookies, post_params, raw, is_https) VALUES($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id;`
	insertResponseQuery = `INSERT INTO responses(request_id, code, message, headers, body) VALUES($1, $2, $3, $4, $5);`
)

func NewProxyRepository(conn *pgx.ConnPool) *ProxyRepository {
	return &ProxyRepository{
		conn: conn,
	}
}
func (p *ProxyRepository) InsertRequest(req *Request) (uint, error) {
	var id uint
	err := p.conn.QueryRow(insertRequestQuery, req.Method, req.Path, req.GetParams, req.Headers, req.Cookies, req.PostParams, req.Raw, req.IsHTTPS).Scan(&id)
	if err != nil {
		return id, errors.Wrap(err, "inserting request error")
	}
	return id, nil

}

func (p *ProxyRepository) InsertResponse(reqID uint, resp *Response) error {
	res, err := p.conn.Exec(insertResponseQuery, reqID, resp.Code, resp.Message, resp.Headers, resp.Body)
	if err != nil {
		return err
	}
	if res.RowsAffected() != 1 {
		return errors.Wrap(err, "inserting response error")
	}
	return nil
}
