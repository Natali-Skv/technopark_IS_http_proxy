package repeater

import (
	"github.com/jackc/pgx"
)

type RepeaterRepository struct {
	conn *pgx.ConnPool
}

const (
	getAllQueries  = `SELECT id, method, path, get_params, headers, cookies, post_params, raw, is_https from requests;`
	getRequestByID = `SELECT id, method, path, get_params, headers, cookies, post_params, raw, is_https from requests WHERE id = $1;`
)

func NewRepeaterRepository(conn *pgx.ConnPool) *RepeaterRepository {
	return &RepeaterRepository{
		conn: conn,
	}
}

func (p *RepeaterRepository) GetAllRequests() ([]RequestResponse, error) {
	rows, err := p.conn.Query(getAllQueries)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]RequestResponse, 0)

	for rows.Next() {
		req := RequestResponse{}
		err = rows.Scan(&req.ID, &req.Method, &req.Path, &req.GetParams, &req.Headers, &req.Cookies, &req.PostParams, &req.Raw, &req.IsHTTPS)
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		res = append(res, req)
	}

	return res, nil
}
func (p *RepeaterRepository) GetRequestByID(id int) (*RequestResponse, error) {
	req := &RequestResponse{}

	err := p.conn.QueryRow(getRequestByID, id).
		Scan(&req.ID, &req.Method, &req.Path, &req.GetParams, &req.Headers, &req.Cookies, &req.PostParams, &req.Raw, &req.IsHTTPS)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return req, nil
}
