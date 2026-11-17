// Package httpclient is a thin typed REST client for the csp2d server. Every
// call has a timeout, so a hung server endpoint fails the test instead of
// blocking it. Calls return the raw Response so that tests can assert on status
// codes and error bodies as well as on decoded payloads.
package httpclient

import (
	"bytes"
	"csp2d/test/support/api"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"time"
)

const DefaultTimeout = 10 * time.Second

// ErrTimeout is wrapped by Do when the request did not complete in time.
var ErrTimeout = errors.New("request timed out (server hung?)")

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTP: &http.Client{Timeout: DefaultTimeout}}
}

// WithTimeout returns a copy of the client with another per-request timeout.
func (c *Client) WithTimeout(d time.Duration) *Client {
	return &Client{BaseURL: c.BaseURL, HTTP: &http.Client{Timeout: d}}
}

// Response is a fully read HTTP response.
type Response struct {
	Method     string
	Path       string
	StatusCode int
	Header     http.Header
	Body       []byte
	Duration   time.Duration
}

func (r *Response) OK() bool { return r.StatusCode >= 200 && r.StatusCode < 300 }

// Decode unmarshals the body into v.
func (r *Response) Decode(v any) error {
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("%s %s: cannot decode body %q: %w", r.Method, r.Path, truncate(r.Body), err)
	}
	return nil
}

// ErrorMessage returns the Echo error message ({"message": ...}) or the raw body.
func (r *Response) ErrorMessage() string {
	var e api.ErrorBody
	if json.Unmarshal(r.Body, &e) == nil && e.Message != "" {
		return e.Message
	}
	return string(r.Body)
}

func (r *Response) String() string {
	return fmt.Sprintf("%s %s -> %d in %s: %s", r.Method, r.Path, r.StatusCode, r.Duration.Round(time.Millisecond), truncate(r.Body))
}

func truncate(b []byte) string {
	const max = 512
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}

// Do sends a request with an optional body and content type.
func (c *Client) Do(method, path string, body []byte, contentType string) (*Response, error) {
	req, err := http.NewRequest(method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	start := time.Now()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, fmt.Errorf("%s %s: %w after %s", method, path, ErrTimeout, time.Since(start).Round(time.Millisecond))
		}
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s %s: reading body: %w", method, path, err)
	}
	return &Response{
		Method: method, Path: path, StatusCode: resp.StatusCode, Header: resp.Header,
		Body: data, Duration: time.Since(start),
	}, nil
}

func (c *Client) doJSON(method, path string, payload any) (*Response, error) {
	if payload == nil {
		return c.Do(method, path, nil, "")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.Do(method, path, body, "application/json")
}

// ---- Endpoints --------------------------------------------------------------

func (c *Client) Heartbeat() (*Response, error) { return c.Do(http.MethodGet, "/heartbeat", nil, "") }

func (c *Client) DefaultParameters() (*Response, error) {
	return c.Do(http.MethodGet, "/api/v1/service/default-parameters", nil, "")
}

// Start posts a typed request.
func (c *Client) Start(req *api.StartRequest) (*Response, error) {
	return c.doJSON(http.MethodPost, "/api/v1/simulation/start", req)
}

// StartRaw posts an arbitrary body (used for malformed-JSON scenarios).
func (c *Client) StartRaw(body []byte) (*Response, error) {
	return c.Do(http.MethodPost, "/api/v1/simulation/start", body, "application/json")
}

func (c *Client) Pause(id string) (*Response, error) {
	return c.Do(http.MethodGet, "/api/v1/simulation/pause/"+id, nil, "")
}

func (c *Client) Resume(id string) (*Response, error) {
	return c.Do(http.MethodGet, "/api/v1/simulation/continue/"+id, nil, "")
}

func (c *Client) Configure(id string, params *api.Parameters) (*Response, error) {
	return c.doJSON(http.MethodPost, "/api/v1/simulation/configure/"+id, &api.ConfigureRequest{Parameters: params})
}

func (c *Client) ConfigureRaw(id string, body []byte) (*Response, error) {
	return c.Do(http.MethodPost, "/api/v1/simulation/configure/"+id, body, "application/json")
}

func (c *Client) Status(id string) (*Response, error) {
	return c.Do(http.MethodGet, "/api/v1/simulation/status/"+id, nil, "")
}

func (c *Client) Result(id string) (*Response, error) {
	return c.Do(http.MethodGet, "/api/v1/simulation/result/"+id, nil, "")
}

func (c *Client) Delete(id string) (*Response, error) {
	return c.Do(http.MethodDelete, "/api/v1/simulation/"+id, nil, "")
}

// PdfImage is one image part of a PDF request.
type PdfImage struct {
	Name string
	Data []byte
}

// GeneratePdf posts a multipart form. requestJSON is sent verbatim as the
// "request" field when non-nil (so malformed JSON can be tested); images are
// sent as "images" files.
func (c *Client) GeneratePdf(requestJSON []byte, images []PdfImage) (*Response, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if requestJSON != nil {
		if err := w.WriteField("request", string(requestJSON)); err != nil {
			return nil, err
		}
	}
	for _, img := range images {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="images"; filename=%q`, img.Name))
		h.Set("Content-Type", "image/png")
		part, err := w.CreatePart(h)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(img.Data); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return c.Do(http.MethodPost, "/api/v1/pdf/generate", buf.Bytes(), w.FormDataContentType())
}

// ---- Typed convenience wrappers ------------------------------------------------

// Expect checks that the call succeeded with the wanted status code and decodes
// the body into v (when v is non-nil).
func Expect(resp *Response, err error, wantStatus int, v any) (*Response, error) {
	if err != nil {
		return resp, err
	}
	if resp.StatusCode != wantStatus {
		return resp, fmt.Errorf("unexpected status: %s", resp)
	}
	if v != nil {
		return resp, resp.Decode(v)
	}
	return resp, nil
}
