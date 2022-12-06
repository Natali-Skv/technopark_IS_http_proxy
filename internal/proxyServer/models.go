package proxyserver

import (
	"net/http"
)

type Map map[string]interface{}

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

func FormRequestData(r *http.Request, dump []byte) *Request {
	req := &Request{
		Method: r.Method,
		Path:   r.URL.Path,
		Raw:    string(dump),
	}
	getParams := Map{}
	for key, value := range r.URL.Query() {
		getParams[key] = getValue(value)
	}
	req.GetParams = getParams

	headers := Map{
		"Host": r.Host,
	}

	for key, value := range r.Header {
		if key == "Cookie" {
			continue
		}
		headers[key] = getValue(value)
	}
	req.Headers = headers

	cookies := Map{}

	for _, value := range r.Cookies() {
		cookies[value.Name] = value.Value
	}
	req.Cookies = cookies

	postParams := Map{}
	_ = r.ParseForm()
	for key, value := range r.PostForm {
		postParams[key] = getValue(value)
	}
	req.PostParams = postParams
	return req
}
func FormResponseData(response *http.Response, body string) *Response {
	if response == nil {
		return nil
	}
	res := &Response{
		Code:    response.StatusCode,
		Message: response.Status,
	}

	headers := Map{}

	for key, value := range response.Header {
		if key == "Cookie" {
			continue
		}
		headers[key] = getValue(value)
	}
	res.Headers = headers
	res.Body = body

	return res

}

func getValue(value []string) interface{} {
	if len(value) == 1 {
		return value[0]
	}
	return value
}
