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

type Resource struct {
	Name     string
	Path     string
	ReadOnly bool
	IsPolicy bool
	IsMeshed bool
}

type Metadata struct {
	Resources []Resource
	Product   string
	Version   string
}

func (m *Metadata) ResourceForPath(path string) string {
	for _, v := range m.Resources {
		if v.Path == path {
			return v.Name
		}
	}
	return ""
}

func (m *Metadata) PathForResource(name string) string {
	for _, v := range m.Resources {
		if v.Name == name {
			return v.Path
		}
	}
	return ""
}

type Client interface {
	HeartBeat(ctx context.Context) (Metadata, error)
	FetchResource(context.Context, string, string, string) ([]byte, error)
	PutResource(context.Context, string, string, string, string) error
	DeleteResource(context.Context, string, string, string) error
}

type ClientImpl struct {
	client   *http.Client
	endpoint string
	token    string
}

func (c *ClientImpl) DeleteResource(ctx context.Context, mesh string, resType string, name string) error {
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

func (c *ClientImpl) HeartBeat(ctx context.Context) (Metadata, error) {
	resp := Metadata{}
	index, err := c.index(ctx)
	if err != nil {
		return resp, fmt.Errorf("failed index request, error=%w", err)
	}
	resp.Product = index["product"].(string)
	resp.Version = index["version"].(string)
	resources, err := c.policies(ctx)
	if err != nil {
		return resp, fmt.Errorf("failed policies request, error=%w", err)
	}
	resp.Resources = resources
	return resp, nil
}

func (c *ClientImpl) index(ctx context.Context) (map[string]interface{}, error) {
	req, err := c.baseRequest(ctx, http.MethodGet, "/", "")
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid http response '%s'", res.Status)
	}
	index := map[string]interface{}{}
	err = json.NewDecoder(res.Body).Decode(&index)
	if err != nil {
		return nil, fmt.Errorf("failed to decode json error='%w'", err)
	}
	return index, nil
}

type PolicyResponse struct {
	Policies []Policy `json:"policies"`
}

type Policy struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	ReadOnly bool   `json:"readOnly"`
}

func (c *ClientImpl) policies(ctx context.Context) ([]Resource, error) {
	req, err := c.baseRequest(ctx, http.MethodGet, "/policies", "")
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid http response '%s'", res.Status)
	}
	resp := PolicyResponse{}
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode json error='%w'", err)
	}
	var out []Resource
	for _, v := range resp.Policies {
		out = append(out, Resource{
			IsPolicy: true,
			Path:     v.Path,
			Name:     v.Name,
			ReadOnly: v.ReadOnly,
		})
	}
	return out, nil
}

func (c *ClientImpl) FetchResource(ctx context.Context, mesh string, resType string, name string) ([]byte, error) {
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

func (c *ClientImpl) PutResource(ctx context.Context, mesh string, resType string, name string, entity string) error {
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
