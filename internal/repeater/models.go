package repeater

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type Map map[string]interface{}

func (p Map) Value() (driver.Value, error) {
	j, err := json.Marshal(p)
	return j, err
}
func (p *Map) Scan(src interface{}) error {
	source, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed")
	}

	var i interface{}
	err := json.Unmarshal(source, &i)
	if err != nil {
		return err
	}

	*p, ok = i.(map[string]interface{})
	if !ok {
		return errors.New("type assertion .(map[string]interface{}) failed")
	}

	return nil
}

type RequestResponse struct {
	ID int64 `json:"id"`
	Request
}

type Request struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	GetParams  Map    `json:"get_params"`
	Headers    Map    `json:"headers"`
	Cookies    Map    `json:"cookies"`
	PostParams Map    `json:"post_params"`
	Raw        string `json:"raw"`
	IsHTTPS    bool   `json:"is_https"`
}
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Headers Map    `json:"headers"`
	Body    string `json:"body"`
	Raw     string `json:"raw"`
	IsHTTPS bool   `json:"is_https"`
}
