package util

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecodedResponseBodyLimit(t *testing.T) {
	const limit = 4096
	for _, tc := range []struct {
		name       string
		compressed bool
		size       int
		limited    bool
	}{
		{"plain exact", false, limit, true}, {"plain overflow", false, limit + 1, true},
		{"gzip exact", true, limit, true}, {"gzip overflow", true, limit + 1, true},
		{"legacy unrestricted", true, limit + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				payload := bytes.Repeat([]byte{'x'}, tc.size)
				if tc.compressed {
					w.Header().Set("Content-Encoding", "gzip")
					gz := gzip.NewWriter(w)
					_, _ = gz.Write(payload)
					_ = gz.Close()
				} else {
					_, _ = w.Write(payload)
				}
			}))
			defer server.Close()
			req := NewGetRequest(server.URL, nil)
			if tc.limited {
				req.Context = WithResponseBodyLimit(context.Background(), limit)
			}
			res, err := ExecuteRequest(req)
			if tc.limited && tc.size > limit {
				if !errors.Is(err, ErrResponseBodyTooLarge) || res == nil || len(res.Body) != 0 {
					t.Fatalf("result=%v err=%v", res, err)
				}
			} else if err != nil || res == nil || len(res.Body) != tc.size {
				t.Fatalf("unexpected response: %v", err)
			}
		})
	}
}

type countedBody struct {
	*bytes.Reader
	read   int
	closed bool
}

func (b *countedBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err
}
func (b *countedBody) Close() error { b.closed = true; return nil }

type bodyRoundTripper func(*http.Request) (*http.Response, error)

func (f bodyRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLimitedResponseDoesNotDrain(t *testing.T) {
	body := &countedBody{Reader: bytes.NewReader(bytes.Repeat([]byte{'x'}, 4096))}
	c := &http.Client{Transport: bodyRoundTripper(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: body, Request: r}, nil
	})}
	req, err := http.NewRequestWithContext(WithResponseBodyLimit(context.Background(), 16), http.MethodGet, "http://example.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := execute(c, req); !errors.Is(err, ErrResponseBodyTooLarge) || body.read != 17 || !body.closed {
		t.Fatalf("read=%d closed=%v err=%v", body.read, body.closed, err)
	}
}

func TestLimitedResponsePropagatesReadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, _ = gz.Write([]byte("data"))
		_ = gz.Close()
		_, _ = w.Write(buf.Bytes()[:buf.Len()-2])
	}))
	defer server.Close()
	req := NewGetRequest(server.URL, nil)
	req.Context = WithResponseBodyLimit(context.Background(), 64)
	if _, err := ExecuteRequest(req); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v", err)
	}
}
