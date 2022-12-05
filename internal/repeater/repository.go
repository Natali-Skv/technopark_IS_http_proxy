package repeater

import (
	"context"
	"errors"

	"github.com/jackc/pgx"
)

type RepeaterRepository struct {
	conn *pgx.ConnPool
}

const (
	insertRequestQuery  = `INSERT INTO requests(method, path, get_params, headers, cookies, post_params, raw, is_https) VALUES($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id;`
	insertResponseQuery = `INSERT INTO responses(request_id, code, message, headers, body) VALUES($1, $2, $3, $4, $5);`
	getAllQueries       = `SELECT id, method, path, get_params, headers, cookies, post_params, raw, is_https from requests;`
	getRequestByID      = `SELECT id, method, path, get_params, headers, cookies, post_params, raw, is_https from requests WHERE id = $1;`
)

func NewProxyRepository(conn *pgx.ConnPool) *RepeaterRepository {
	return &RepeaterRepository{
		conn: conn,
	}
}
func (p *RepeaterRepository) InsertRequest(req *models.Request) (int, error) {
	id := -1
	err := p.conn.Conn.QueryRow(context.Background(), insertRequestQuery, req.Method, req.Path, req.GetParams, req.Headers, req.Cookies, req.PostParams, req.Raw, req.IsHTTPS).Scan(&id)
	if err != nil {
		return id, err
	}
	return id, nil

}

func (p *RepeaterRepository) GetAllRequests() ([]models.RequestResponse, error) {
	rows, err := p.conn.Conn.Query(context.Background(), getAllQueries)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]models.RequestResponse, 0)

	for rows.Next() {
		req := models.RequestResponse{}
		err = rows.Scan(&req.ID, &req.Method, &req.Path, &req.GetParams, &req.Headers, &req.Cookies, &req.PostParams, &req.Raw, &req.IsHTTPS)
		if err != nil {
			return nil, err
		}
		res = append(res, req)
	}

	return res, nil
}
func (p *RepeaterRepository) GetRequestByID(id int) (*models.RequestResponse, error) {
	req := &models.RequestResponse{}

	err := p.conn.Conn.QueryRow(context.Background(), getRequestByID, id).
		Scan(&req.ID, &req.Method, &req.Path, &req.GetParams, &req.Headers, &req.Cookies, &req.PostParams, &req.Raw, &req.IsHTTPS)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (p *RepeaterRepository) InsertResponse(reqID int, resp *models.Response) error {
	res, err := p.conn.Conn.Exec(context.Background(), insertResponseQuery, reqID, resp.Code, resp.Message, resp.Headers, resp.Body)
	if err != nil {
		return err
	}
	if res.RowsAffected() != 1 {
		return errors.New("internal server error")
	}
	return nil
}
