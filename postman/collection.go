package postman

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// Collection represents a collection of requests
type Collection struct {
	Info CollectionInfo
	Item []CollectionItem
	Auth CollectionAuth
}

// CollectionInfo represents info about a collection
type CollectionInfo struct {
	ID     string `json:"_postman_id"`
	Name   string
	Schema string
}

// CollectionAuth defines the authentication headers for the collection
type CollectionAuth struct {
	Type   string
	Bearer CollectionBearerAuth
}

// CollectionBearerAuth represents bearer token auth
type CollectionBearerAuth struct {
	Key   string
	Value string
	Type  string
}

// CollectionItem represents a request/response in a collection
type CollectionItem struct {
	Name     string
	Request  Request
	Response []Response
}

// Request represents a request to the endpoint
type Request struct {
	Method string
	Header []Header
	Body   Body `json:"Body,omitempty"`
	URL    URL
}

// Header represents a header
type Header struct {
	Key   string
	Name  string
	Value string
	Type  string
}

// Body represents a body
type Body struct {
	Mode string
	Raw  string
}

// Response describes a response
type Response struct {
	Mode   string
	Raw    string
	Status int
}

// URL represents a URL
type URL struct {
	Raw  string
	Host []string
	Port string
	Path []string
}

// NewCollection returns a new Collection
func NewCollection(name string, items []CollectionItem, auth *CollectionAuth) *Collection {
	info := CollectionInfo{
		Name:   name,
		Schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
	}

	collection := Collection{
		Info: info,
		Item: items,
	}

	if auth != nil {
		collection.Auth = *auth
	}

	return &collection
}

// ItemWithName gets an item with a particular name
func (c *Collection) ItemWithName(name string) *CollectionItem {
	for i, itm := range c.Item {
		if itm.Name == name {
			return &c.Item[i]
		}
	}

	return nil
}

// RequestFromHTTP converts an http request to a postman request
func RequestFromHTTP(r *http.Request) (*Request, error) {
	req := Request{
		Method: r.Method,
		URL: URL{
			Raw:  r.URL.String(),
			Host: strings.Split(r.URL.Hostname(), "."),
			Port: r.URL.Port(),
			Path: strings.Split(r.URL.Path, "/"),
		},
	}

	headers := []Header{}
	for k, v := range r.Header {
		for _, val := range v {
			header := Header{
				Key:   k,
				Name:  k,
				Value: val,
				Type:  "text",
			}

			headers = append(headers, header)
		}
	}

	req.Header = headers

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ReadAll")
	}

	// create a copy for ourselves
	bodyCopy := ioutil.NopCloser(bytes.NewBuffer(buf))

	// put a second copy back into the request for the actual handler to use
	r.Body = ioutil.NopCloser(bytes.NewBuffer(buf))

	body, err := ioutil.ReadAll(bodyCopy)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ReadAll")
	}

	if len(body) > 0 {
		req.Body = Body{
			Mode: "raw",
			Raw:  string(body),
		}
	}

	return &req, nil
}

// ToHTTPRequest converts a postman request to an http request
func (r *Request) ToHTTPRequest(vars map[string]string) *http.Request {
	tmplAddr, err := SubstVars(r.URL.Raw, vars)
	if err != nil {
		tmplAddr = r.URL.Raw
	}

	tmplBody, err := SubstVars(r.Body.Raw, vars)
	if err != nil {
		tmplBody = r.Body.Raw
	}

	req, err := http.NewRequest(r.Method, tmplAddr, bytes.NewBuffer([]byte(tmplBody)))

	if err != nil {
		return nil
	}

	if vars != nil {
		for _, h := range r.Header {
			tmplKey, err := SubstVars(h.Key, vars)
			if err != nil {
				tmplKey = h.Key
			}

			tmplVal, err := SubstVars(h.Value, vars)
			if err != nil {
				tmplVal = h.Value
			}

			req.Header.Add(tmplKey, tmplVal)
		}

	} else {
		for _, h := range r.Header {
			req.Header.Add(h.Key, h.Value)
		}
	}

	return req
}

// InflateEnvironmentVariables for the Response
func (r *Response) InflateEnvironmentVariables(vars map[string]string) (*Response, error) {

	inflatedMode, err := SubstVars(r.Mode, vars)
	if err != nil {
		return nil, errors.New("unable in inflate response mode")
	}
	inflatedRaw, err := SubstVars(r.Raw, vars)
	if err != nil {
		return nil, errors.New("unable in inflate response raw")
	}

	return &Response{inflatedMode, inflatedRaw, r.Status}, nil
}

// ToInterface unmarshals a response into an interface
func (r *Response) ToInterface(out interface{}) error {
	if out == nil {
		return nil
	}

	if err := json.Unmarshal([]byte(r.Raw), out); err != nil {
		return err
	}

	return nil
}
