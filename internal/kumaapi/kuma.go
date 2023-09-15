package kumaapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type IndexResponse struct {
}

type Client interface {
	HeartBeat(ctx context.Context) (IndexResponse, error)
	FetchPolicy(context.Context, string, string, string) ([]byte, error)
	PutPolicy(context.Context, string, string, string, string) error
	DeletePolicy(ctx context.Context, valueString string, s string, valueString2 string) error
}

type ClientImpl struct {
	client   *http.Client
	endpoint string
	token    string
}

func (c *ClientImpl) DeletePolicy(ctx context.Context, mesh string, resType string, name string) error {
	path := fmt.Sprintf("/meshes/%s/%s/%s", mesh, resType, name)
	req, err := c.baseRequest(ctx, http.MethodDelete, path, "")
	if err != nil {
		return fmt.Errorf("couldn't create delete request error='%w'", err)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		return nil
	default:
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("invalid http response '%s' for GET '%s' request. Response: '%s'", res.Status, path, string(b))
	}
}

func (c *ClientImpl) baseRequest(ctx context.Context, method string, path string, data string) (*http.Request, error) {
	var b io.Reader = nil
	if data != "" {
		b = bytes.NewBuffer([]byte(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, b)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	return req, nil
}

func (c *ClientImpl) HeartBeat(ctx context.Context) (IndexResponse, error) {
	resp := IndexResponse{}
	req, err := c.baseRequest(ctx, http.MethodGet, "/", "")
	if err != nil {
		return resp, fmt.Errorf("couldn't create heartbeat request error='%w'", err)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return IndexResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("invalid http response '%s' for heartbeat request", res.Status)
	}
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return resp, fmt.Errorf("failed to decode json for heartbeat request error='%w'", err)
	}
	return resp, nil
}

func (c *ClientImpl) FetchPolicy(ctx context.Context, mesh string, resType string, name string) ([]byte, error) {
	path := fmt.Sprintf("/meshes/%s/%s/%s", mesh, resType, name)
	req, err := c.baseRequest(ctx, http.MethodGet, path, "")
	if err != nil {
		return nil, fmt.Errorf("couldn't create request for request error='%w'", err)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		return b, nil
	case http.StatusNotFound:
		return nil, nil
	default:
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("invalid http response '%s' for GET '%s' request. Response: '%s'", res.Status, path, string(b))
	}
}

func (c *ClientImpl) PutPolicy(ctx context.Context, mesh string, resType string, name string, entity string) error {
	path := fmt.Sprintf("/meshes/%s/%s/%s", mesh, resType, name)
	req, err := c.baseRequest(ctx, http.MethodPut, path, entity)
	if err != nil {
		return fmt.Errorf("couldn't create request for request error='%w'", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return nil
	default:
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("invalid http response '%s' for PUT '%s' request. Response: '%s'", res.Status, path, string(b))
	}

}

func NewClient(endpoint string, token string) Client {
	if strings.HasSuffix("/", endpoint) {
		endpoint = strings.TrimRight(endpoint, "/")
	}
	return &ClientImpl{
		client:   http.DefaultClient,
		endpoint: endpoint,
		token:    token,
	}
}
