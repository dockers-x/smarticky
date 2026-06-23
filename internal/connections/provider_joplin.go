package connections

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type joplinProvider struct {
	endpoint string
	token    string
	client   *http.Client
}

type joplinList[T any] struct {
	Items   []T  `json:"items"`
	HasMore bool `json:"has_more"`
}

type joplinFolder struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	ParentID string `json:"parent_id"`
}

type joplinNote struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	ParentID    string `json:"parent_id"`
	CreatedTime int64  `json:"created_time"`
	UpdatedTime int64  `json:"updated_time"`
}

func newJoplinProvider(endpoint, token string, httpClient *http.Client) Provider {
	return &joplinProvider{
		endpoint: endpoint,
		token:    token,
		client:   httpClient,
	}
}

func (p *joplinProvider) Test(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint+"/ping", nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("joplin ping failed: %s", resp.Status)
	}
	if !strings.Contains(string(body), "JoplinClipperServer") {
		return errorsNew("joplin clipper server not found")
	}

	var folders joplinList[joplinFolder]
	return p.doJSON(ctx, http.MethodGet, "/folders", url.Values{
		"limit":  {"1"},
		"fields": {"id,title,parent_id"},
	}, nil, &folders)
}

func (p *joplinProvider) ListTargets(ctx context.Context) ([]Target, error) {
	var targets []Target
	page := 1
	for {
		var response joplinList[joplinFolder]
		err := p.doJSON(ctx, http.MethodGet, "/folders", url.Values{
			"page":   {strconv.Itoa(page)},
			"limit":  {"100"},
			"fields": {"id,title,parent_id"},
		}, nil, &response)
		if err != nil {
			return nil, err
		}
		for _, folder := range response.Items {
			targets = append(targets, Target{
				ID:       folder.ID,
				Name:     folder.Title,
				Kind:     "folder",
				ParentID: folder.ParentID,
			})
		}
		if !response.HasMore {
			break
		}
		page++
	}
	return targets, nil
}

func (p *joplinProvider) ImportNotes(ctx context.Context, targetID string, limit int) ([]RemoteNote, error) {
	limit = clampLimit(limit)
	foldersByID := map[string]string{}
	if targets, err := p.ListTargets(ctx); err == nil {
		for _, target := range targets {
			foldersByID[target.ID] = target.Name
		}
	}

	endpoint := "/notes"
	if strings.TrimSpace(targetID) != "" {
		endpoint = "/folders/" + url.PathEscape(strings.TrimSpace(targetID)) + "/notes"
	}

	var notes []RemoteNote
	page := 1
	for len(notes) < limit {
		var response joplinList[joplinNote]
		err := p.doJSON(ctx, http.MethodGet, endpoint, url.Values{
			"page":   {strconv.Itoa(page)},
			"limit":  {strconv.Itoa(minInt(100, limit-len(notes)))},
			"fields": {"id,title,body,parent_id,created_time,updated_time"},
		}, nil, &response)
		if err != nil {
			return nil, err
		}
		for _, item := range response.Items {
			notes = append(notes, RemoteNote{
				ExternalID: item.ID,
				TargetID:   item.ParentID,
				TargetName: foldersByID[item.ParentID],
				Title:      titleOrUntitled(item.Title),
				Content:    item.Body,
				CreatedAt:  parseJoplinTime(item.CreatedTime),
				UpdatedAt:  parseJoplinTime(item.UpdatedTime),
			})
			if len(notes) >= limit {
				break
			}
		}
		if !response.HasMore {
			break
		}
		page++
	}
	return notes, nil
}

func (p *joplinProvider) PushNote(ctx context.Context, input PushInput) (PushResult, error) {
	body := map[string]string{
		"title": input.Title,
		"body":  input.Content,
	}
	if strings.TrimSpace(input.TargetID) != "" {
		body["parent_id"] = strings.TrimSpace(input.TargetID)
	}

	var saved joplinNote
	if input.ExistingExternalID != "" {
		err := p.doJSON(ctx, http.MethodPut, "/notes/"+url.PathEscape(input.ExistingExternalID), nil, body, &saved)
		if err != nil {
			return PushResult{}, err
		}
	} else {
		err := p.doJSON(ctx, http.MethodPost, "/notes", nil, body, &saved)
		if err != nil {
			return PushResult{}, err
		}
	}
	return PushResult{
		ExternalID: saved.ID,
		TargetID:   saved.ParentID,
	}, nil
}

func (p *joplinProvider) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	target, err := url.Parse(p.endpoint + path)
	if err != nil {
		return err
	}
	values := target.Query()
	values.Set("token", p.token)
	for key, list := range query {
		for _, value := range list {
			values.Add(key, value)
		}
	}
	target.RawQuery = values.Encode()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("joplin request failed: %s %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func parseJoplinTime(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	parsed := time.UnixMilli(value).UTC()
	return &parsed
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func errorsNew(message string) error {
	return fmt.Errorf("%s", message)
}
