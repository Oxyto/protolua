package protoflux

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const DefaultWikiAPI = "https://wiki.resonite.com/api.php"

var wikiHTTPClient = http.DefaultClient

type wikiCategoryResponse struct {
	Continue map[string]string `json:"continue"`
	Query    struct {
		CategoryMembers []struct {
			Title string `json:"title"`
		} `json:"categorymembers"`
	} `json:"query"`
}

func FetchWikiAll(ctx context.Context, apiURL string) ([]Node, error) {
	if apiURL == "" {
		apiURL = DefaultWikiAPI
	}
	nodes := []Node{}
	seen := map[string]bool{}
	continueValue := ""
	for {
		requestURL, err := wikiCategoryURL(apiURL, continueValue)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := wikiHTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		var decoded wikiCategoryResponse
		err = json.NewDecoder(resp.Body).Decode(&decoded)
		closeErr := resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("wiki API returned %s", resp.Status)
		}
		for _, member := range decoded.Query.CategoryMembers {
			name := strings.TrimPrefix(member.Title, "ProtoFlux:")
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			nodes = append(nodes, Node{Name: name, Canonical: "ProtoFlux:" + name})
		}
		next := decoded.Continue["cmcontinue"]
		if next == "" {
			break
		}
		continueValue = next
	}
	return nodes, nil
}

func wikiCategoryURL(apiURL, continueValue string) (string, error) {
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("action", "query")
	query.Set("list", "categorymembers")
	query.Set("cmtitle", "Category:ProtoFlux:All")
	query.Set("cmlimit", "max")
	query.Set("format", "json")
	if continueValue != "" {
		query.Set("cmcontinue", continueValue)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
