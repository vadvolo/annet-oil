package jira

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/andygrunwald/go-jira"
)

type Config struct {
	URL        string
	Email      string
	Token      string
	ProjectKey string
	IssueType  string
}

type Client struct {
	client     *jira.Client
	config     Config
	httpClient HTTPClient
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewClient(cfg Config) (*Client, error) {
	tp := jira.BasicAuthTransport{
		Username:  cfg.Email,
		Password:  cfg.Token,
		Transport: http.DefaultTransport,
	}

	baseURL := cfg.URL
	if baseURL != "" && baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}

	client, err := jira.NewClient(tp.Client(), baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create jira client: %w", err)
	}

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

func NewClientWithHTTP(cfg Config, httpClient HTTPClient) (*Client, error) {
	return &Client{
		config:     cfg,
		httpClient: httpClient,
	}, nil
}

type CreateTicketParams struct {
	Summary     string
	Description string
	Labels      []string
	Priority    string
	Assignee    string
}

func (c *Client) CreateTicket(ctx context.Context, params CreateTicketParams) (*jira.Issue, error) {
	issue := &jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: c.config.ProjectKey,
			},
			Summary:     params.Summary,
			Description: params.Description,
			Type: jira.IssueType{
				Name: c.config.IssueType,
			},
		},
	}

	if len(params.Labels) > 0 {
		issue.Fields.Labels = params.Labels
	}

	if params.Priority != "" {
		issue.Fields.Priority = &jira.Priority{Name: params.Priority}
	}

	if params.Assignee != "" {
		issue.Fields.Assignee = &jira.User{Name: params.Assignee}
	}

	created, resp, err := c.client.Issue.CreateWithContext(ctx, issue)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("failed to create issue (HTTP %d): %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return created, nil
}

func (c *Client) GetIssue(ctx context.Context, issueKey string) (*jira.Issue, error) {
	issue, _, err := c.client.Issue.GetWithContext(ctx, issueKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}
	return issue, nil
}

func (c *Client) GetIssueStatus(ctx context.Context, issueKey string) (string, error) {
	issue, err := c.GetIssue(ctx, issueKey)
	if err != nil {
		return "", err
	}
	return issue.Fields.Status.Name, nil
}

func (c *Client) AddComment(ctx context.Context, issueKey, body string) error {
	comment := &jira.Comment{Body: body}
	_, _, err := c.client.Issue.AddCommentWithContext(ctx, issueKey, comment)
	if err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}
	return nil
}

func (c *Client) UpdateDescription(ctx context.Context, issueKey, description string) error {
	issue := &jira.Issue{
		Key: issueKey,
		Fields: &jira.IssueFields{
			Description: description,
		},
	}

	_, _, err := c.client.Issue.UpdateWithContext(ctx, issue)
	if err != nil {
		return fmt.Errorf("failed to update description: %w", err)
	}
	return nil
}

func (c *Client) TransitionTo(ctx context.Context, issueKey, targetStatus string) error {
	transitions, _, err := c.client.Issue.GetTransitionsWithContext(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("failed to get transitions: %w", err)
	}

	var transitionID string
	for _, t := range transitions {
		if t.To.Name == targetStatus {
			transitionID = t.ID
			break
		}
	}

	if transitionID == "" {
		return fmt.Errorf("transition to '%s' not available", targetStatus)
	}

	_, err = c.client.Issue.DoTransitionWithContext(ctx, issueKey, transitionID)
	if err != nil {
		return fmt.Errorf("failed to transition: %w", err)
	}

	return nil
}

func (c *Client) GetAvailableTransitions(ctx context.Context, issueKey string) ([]string, error) {
	transitions, _, err := c.client.Issue.GetTransitionsWithContext(ctx, issueKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get transitions: %w", err)
	}

	result := make([]string, len(transitions))
	for i, t := range transitions {
		result[i] = t.To.Name
	}
	return result, nil
}

func (c *Client) AssignTo(ctx context.Context, issueKey, assignee string) error {
	issue := &jira.Issue{
		Key: issueKey,
		Fields: &jira.IssueFields{
			Assignee: &jira.User{Name: assignee},
		},
	}

	_, _, err := c.client.Issue.UpdateWithContext(ctx, issue)
	if err != nil {
		return fmt.Errorf("failed to assign: %w", err)
	}
	return nil
}

func (c *Client) SetPriority(ctx context.Context, issueKey, priority string) error {
	issue := &jira.Issue{
		Key: issueKey,
		Fields: &jira.IssueFields{
			Priority: &jira.Priority{Name: priority},
		},
	}

	_, _, err := c.client.Issue.UpdateWithContext(ctx, issue)
	if err != nil {
		return fmt.Errorf("failed to set priority: %w", err)
	}
	return nil
}

func (c *Client) Search(ctx context.Context, jql string) ([]jira.Issue, error) {
	issues, _, err := c.client.Issue.SearchWithContext(ctx, jql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	return issues, nil
}

func (c *Client) CloseTicket(ctx context.Context, issueKey, resolution string) error {
	transitions, _, err := c.client.Issue.GetTransitionsWithContext(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("failed to get transitions: %w", err)
	}

	var closeID string
	for _, t := range transitions {
		if t.To.Name == "Closed" || t.To.Name == "Done" {
			closeID = t.ID
			break
		}
	}

	if closeID == "" {
		return fmt.Errorf("no close transition available")
	}

	_, err = c.client.Issue.DoTransitionWithContext(ctx, issueKey, closeID)
	if err != nil {
		return fmt.Errorf("failed to close: %w", err)
	}

	if resolution != "" {
		return c.AddComment(ctx, issueKey, fmt.Sprintf("Closed with resolution: %s", resolution))
	}

	return nil
}

type DeployInfo struct {
	User   string
	Hosts  []string
	Result string
	Time   time.Time
}

func (c *Client) AddDeployComment(ctx context.Context, issueKey string, info DeployInfo) error {
	comment := fmt.Sprintf(
		"*Deployment Executed*\n\n"+
			"||Field||Value||\n"+
			"|User|%s|\n"+
			"|Hosts|%v|\n"+
			"|Result|%s|\n"+
			"|Time|%s|",
		info.User, info.Hosts, info.Result, info.Time.Format(time.RFC3339))
	return c.AddComment(ctx, issueKey, comment)
}

func (c *Client) AddDiffComment(ctx context.Context, issueKey, device, diff string) error {
	comment := fmt.Sprintf(
		"*Configuration Diff for %s*\n\n{code}\n%s\n{code}",
		device, diff)
	return c.AddComment(ctx, issueKey, comment)
}

func (c *Client) GetCurrentUser(ctx context.Context) (*jira.User, error) {
	user, _, err := c.client.User.GetSelfWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}
	return user, nil
}
