package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestAddPRComment(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		repo       string
		prNumber   int
		body       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful comment",
			owner:      "testorg",
			repo:       "testrepo",
			prNumber:   42,
			body:       "Test comment",
			statusCode: 201,
			wantErr:    false,
		},
		{
			name:       "PR not found",
			owner:      "testorg",
			repo:       "testrepo",
			prNumber:   999,
			body:       "Test comment",
			statusCode: 404,
			wantErr:    true,
		},
		{
			name:       "unauthorized",
			owner:      "testorg",
			repo:       "testrepo",
			prNumber:   42,
			body:       "Test comment",
			statusCode: 401,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockHTTPClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					if req.Method != "POST" {
						t.Errorf("expected POST, got %s", req.Method)
					}

					auth := req.Header.Get("Authorization")
					if auth != "Bearer test-token" {
						t.Errorf("expected Bearer test-token, got %s", auth)
					}

					response := `{"id": 123, "body": "Test comment", "created_at": "2024-01-15T10:30:00Z", "user": {"login": "testuser"}}`

					return &http.Response{
						StatusCode: tt.statusCode,
						Body:       io.NopCloser(strings.NewReader(response)),
					}, nil
				},
			}

			client := NewClientWithHTTP(Config{Token: "test-token"}, mock)

			comment, err := client.AddPRComment(context.Background(), tt.owner, tt.repo, tt.prNumber, tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddPRComment() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && comment == nil {
				t.Error("expected comment, got nil")
			}
		})
	}
}

func TestAddDeployComment(t *testing.T) {
	var capturedBody map[string]string

	mock := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			json.Unmarshal(body, &capturedBody)

			response := `{"id": 123, "body": "test", "created_at": "2024-01-15T10:30:00Z", "user": {"login": "bot"}}`
			return &http.Response{
				StatusCode: 201,
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		},
	}

	client := NewClientWithHTTP(Config{Token: "test-token"}, mock)

	info := DeployInfo{
		User:   "admin",
		Hosts:  []string{"router-1", "router-2"},
		Result: "success",
		Time:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	_, err := client.AddDeployComment(context.Background(), "org", "repo", 42, info)
	if err != nil {
		t.Errorf("AddDeployComment() error = %v", err)
	}

	body := capturedBody["body"]
	if !strings.Contains(body, "admin") {
		t.Error("comment should contain user name")
	}
	if !strings.Contains(body, "router-1") {
		t.Error("comment should contain hosts")
	}
	if !strings.Contains(body, "success") {
		t.Error("comment should contain result")
	}
}

func TestGetPullRequest(t *testing.T) {
	mock := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			if req.Method != "GET" {
				t.Errorf("expected GET, got %s", req.Method)
			}

			response := `{
				"number": 42,
				"title": "Test PR",
				"state": "open",
				"head": {"ref": "feature-branch", "sha": "abc123"},
				"base": {"ref": "main", "sha": "def456"}
			}`

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		},
	}

	client := NewClientWithHTTP(Config{Token: "test-token"}, mock)

	pr, err := client.GetPullRequest(context.Background(), "org", "repo", 42)
	if err != nil {
		t.Fatalf("GetPullRequest() error = %v", err)
	}

	if pr.Number != 42 {
		t.Errorf("expected number 42, got %d", pr.Number)
	}
	if pr.Title != "Test PR" {
		t.Errorf("expected title 'Test PR', got %s", pr.Title)
	}
	if pr.State != "open" {
		t.Errorf("expected state 'open', got %s", pr.State)
	}
	if pr.Head.Ref != "feature-branch" {
		t.Errorf("expected head ref 'feature-branch', got %s", pr.Head.Ref)
	}
}

func TestGetIssue(t *testing.T) {
	mock := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			if req.Method != "GET" {
				t.Errorf("expected GET, got %s", req.Method)
			}

			response := `{
				"number": 123,
				"title": "Test Issue",
				"state": "open",
				"body": "Issue description"
			}`

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		},
	}

	client := NewClientWithHTTP(Config{Token: "test-token"}, mock)

	issue, err := client.GetIssue(context.Background(), "org", "repo", 123)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}

	if issue.Number != 123 {
		t.Errorf("expected number 123, got %d", issue.Number)
	}
	if issue.Title != "Test Issue" {
		t.Errorf("expected title 'Test Issue', got %s", issue.Title)
	}
	if issue.State != "open" {
		t.Errorf("expected state 'open', got %s", issue.State)
	}
}

func TestClientWithCustomBaseURL(t *testing.T) {
	mock := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.String(), "https://github.company.com/api/v3") {
				t.Errorf("expected custom base URL, got %s", req.URL.String())
			}

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"number": 1, "title": "Test", "state": "open"}`)),
			}, nil
		},
	}

	client := NewClientWithHTTP(Config{
		Token:   "test-token",
		BaseURL: "https://github.company.com/api/v3",
	}, mock)

	_, err := client.GetIssue(context.Background(), "org", "repo", 1)
	if err != nil {
		t.Errorf("GetIssue() error = %v", err)
	}
}
