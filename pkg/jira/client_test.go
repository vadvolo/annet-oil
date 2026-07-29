package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andygrunwald/go-jira"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)

	tp := jira.BasicAuthTransport{
		Username:  "test@example.com",
		Password:  "test-token",
		Transport: http.DefaultTransport,
	}

	client, err := jira.NewClient(tp.Client(), server.URL+"/")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return server, &Client{
		client: client,
		config: Config{
			URL:        server.URL,
			Email:      "test@example.com",
			Token:      "test-token",
			ProjectKey: "TEST",
			IssueType:  "Task",
		},
	}
}

func TestCreateTicket(t *testing.T) {
	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/rest/api/2/issue" {
			response := map[string]interface{}{
				"id":   "12345",
				"key":  "TEST-123",
				"self": "https://test.atlassian.net/rest/api/2/issue/12345",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	issue, err := client.CreateTicket(context.Background(), CreateTicketParams{
		Summary:     "Test RFC",
		Description: "Test description",
		Labels:      []string{"rfc", "network"},
	})

	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	if issue.Key != "TEST-123" {
		t.Errorf("expected key TEST-123, got %s", issue.Key)
	}
}

func TestGetIssue(t *testing.T) {
	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/rest/api/2/issue/TEST-123" {
			response := map[string]interface{}{
				"key": "TEST-123",
				"fields": map[string]interface{}{
					"summary":     "Test issue",
					"description": "Test description",
					"status": map[string]interface{}{
						"name": "Open",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	issue, err := client.GetIssue(context.Background(), "TEST-123")
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}

	if issue.Key != "TEST-123" {
		t.Errorf("expected key TEST-123, got %s", issue.Key)
	}
	if issue.Fields.Summary != "Test issue" {
		t.Errorf("expected summary 'Test issue', got %s", issue.Fields.Summary)
	}
}

func TestAddComment(t *testing.T) {
	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/rest/api/2/issue/TEST-123/comment" {
			response := map[string]interface{}{
				"id":   "10001",
				"body": "Test comment",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	err := client.AddComment(context.Background(), "TEST-123", "Test comment")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
}

func TestGetAvailableTransitions(t *testing.T) {
	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/rest/api/2/issue/TEST-123/transitions" {
			response := map[string]interface{}{
				"transitions": []map[string]interface{}{
					{"id": "11", "to": map[string]string{"name": "In Progress"}},
					{"id": "21", "to": map[string]string{"name": "Done"}},
					{"id": "31", "to": map[string]string{"name": "Closed"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	transitions, err := client.GetAvailableTransitions(context.Background(), "TEST-123")
	if err != nil {
		t.Fatalf("GetAvailableTransitions() error = %v", err)
	}

	if len(transitions) != 3 {
		t.Errorf("expected 3 transitions, got %d", len(transitions))
	}
}

func TestAddDeployComment(t *testing.T) {
	var capturedBody string

	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/rest/api/2/issue/TEST-123/comment" {
			var req map[string]string
			json.NewDecoder(r.Body).Decode(&req)
			capturedBody = req["body"]

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "10001"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	err := client.AddDeployComment(context.Background(), "TEST-123", DeployInfo{
		User:   "admin",
		Hosts:  []string{"router-1", "router-2"},
		Result: "success",
		Time:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	})

	if err != nil {
		t.Fatalf("AddDeployComment() error = %v", err)
	}

	if capturedBody == "" {
		t.Error("comment body should not be empty")
	}
}
