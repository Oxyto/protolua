package protoflux

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFetchWikiAllHandlesPagination(t *testing.T) {
	calls := 0
	previous := wikiHTTPClient
	wikiHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body := `{
			"continue": {"cmcontinue": "next"},
			"query": {"categorymembers": [{"title": "ProtoFlux:Write"}]}
		}`
		if req.URL.Query().Get("cmcontinue") != "" {
			body = `{
				"query": {"categorymembers": [{"title": "ProtoFlux:ReadDynamicVariable"}]}
			}`
		}
		return &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}, nil
	})}
	defer func() { wikiHTTPClient = previous }()

	nodes, err := FetchWikiAll(context.Background(), "https://example.test/api.php")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected two API calls, got %d", calls)
	}
	if len(nodes) != 2 || nodes[0].Canonical != "ProtoFlux:Write" || nodes[1].Canonical != "ProtoFlux:ReadDynamicVariable" {
		t.Fatalf("unexpected nodes: %#v", nodes)
	}
}
