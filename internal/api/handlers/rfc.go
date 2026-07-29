package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/config"
	"annet-oil/internal/logging"
	"annet-oil/pkg/jira"
)

type RFCHandler struct {
	jiraClient *jira.Client
	enabled    bool
}

func NewRFCHandler(cfg config.IntegrationsConfig) http.Handler {
	r := chi.NewRouter()

	handler := &RFCHandler{
		enabled: cfg.Jira.Enabled,
	}

	if cfg.Jira.Enabled && cfg.Jira.URL != "" {
		client, err := jira.NewClient(jira.Config{
			URL:        cfg.Jira.URL,
			Email:      cfg.Jira.Email,
			Token:      cfg.Jira.Token,
			ProjectKey: cfg.Jira.ProjectKey,
			IssueType:  cfg.Jira.IssueType,
		})
		if err != nil {
			logging.Error("Failed to create Jira client", "error", err)
		} else {
			handler.jiraClient = client
		}
	}

	r.Post("/create", handler.createRFC)
	r.Post("/comment", handler.postComment)
	r.Get("/status/{ticketKey}", handler.getStatus)
	r.Post("/submit/{ticketKey}", handler.submitForReview)
	r.Post("/close/{ticketKey}", handler.closeRFC)
	r.Post("/deploy-comment/{ticketKey}", handler.addDeployComment)

	return r
}

type CreateRFCRequest struct {
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Devices     []string `json:"devices"`
	Priority    string   `json:"priority,omitempty"`
}

type CreateRFCResponse struct {
	TicketKey string `json:"ticket_key"`
	URL       string `json:"url"`
}

func (h *RFCHandler) createRFC(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.jiraClient == nil {
		http.Error(w, "Jira integration not configured", http.StatusServiceUnavailable)
		return
	}

	var req CreateRFCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	description := req.Description
	if len(req.Devices) > 0 {
		description += "\n\n*Affected Devices:*\n"
		for _, d := range req.Devices {
			description += "* " + d + "\n"
		}
	}

	issue, err := h.jiraClient.CreateTicket(r.Context(), jira.CreateTicketParams{
		Summary:     req.Summary,
		Description: description,
		Labels:      []string{"rfc", "network-change", "annet-oil"},
		Priority:    req.Priority,
	})
	if err != nil {
		logging.Error("Failed to create RFC ticket", "error", err)
		http.Error(w, "Failed to create RFC ticket: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateRFCResponse{
		TicketKey: issue.Key,
		URL:       issue.Self,
	})
}

type PostCommentRequest struct {
	TicketKey string `json:"ticket_key"`
	Comment   string `json:"comment"`
}

// postComment adds a free-form comment to an RFC ticket. The comment body may be
// anything (a config diff, a note, a status update); Jira wiki markup such as
// {code}...{code} can be included by the caller.
func (h *RFCHandler) postComment(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.jiraClient == nil {
		http.Error(w, "Jira integration not configured", http.StatusServiceUnavailable)
		return
	}

	var req PostCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TicketKey == "" || req.Comment == "" {
		http.Error(w, "ticket_key and comment are required", http.StatusBadRequest)
		return
	}

	if err := h.jiraClient.AddComment(r.Context(), req.TicketKey, req.Comment); err != nil {
		logging.Error("Failed to post comment", "ticket", req.TicketKey, "error", err)
		http.Error(w, "Failed to post comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type RFCStatusResponse struct {
	TicketKey   string   `json:"ticket_key"`
	Summary     string   `json:"summary"`
	Status      string   `json:"status"`
	Transitions []string `json:"transitions"`
}

func (h *RFCHandler) getStatus(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.jiraClient == nil {
		http.Error(w, "Jira integration not configured", http.StatusServiceUnavailable)
		return
	}

	ticketKey := chi.URLParam(r, "ticketKey")

	issue, err := h.jiraClient.GetIssue(r.Context(), ticketKey)
	if err != nil {
		http.Error(w, "Failed to get ticket: "+err.Error(), http.StatusInternalServerError)
		return
	}

	transitions, _ := h.jiraClient.GetAvailableTransitions(r.Context(), ticketKey)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RFCStatusResponse{
		TicketKey:   issue.Key,
		Summary:     issue.Fields.Summary,
		Status:      issue.Fields.Status.Name,
		Transitions: transitions,
	})
}

type SubmitRequest struct {
	Comment string `json:"comment,omitempty"`
}

func (h *RFCHandler) submitForReview(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.jiraClient == nil {
		http.Error(w, "Jira integration not configured", http.StatusServiceUnavailable)
		return
	}

	ticketKey := chi.URLParam(r, "ticketKey")

	var req SubmitRequest
	json.NewDecoder(r.Body).Decode(&req)

	transitions, err := h.jiraClient.GetAvailableTransitions(r.Context(), ticketKey)
	if err != nil {
		http.Error(w, "Failed to get transitions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	reviewStatuses := []string{"In Review", "Pending Approval", "Review", "In Progress"}
	var targetStatus string
	for _, t := range transitions {
		for _, rs := range reviewStatuses {
			if strings.EqualFold(t, rs) {
				targetStatus = t
				break
			}
		}
		if targetStatus != "" {
			break
		}
	}

	if targetStatus != "" {
		if err := h.jiraClient.TransitionTo(r.Context(), ticketKey, targetStatus); err != nil {
			logging.Warn("Failed to transition ticket", "ticket", ticketKey, "error", err)
		}
	}

	if req.Comment != "" {
		h.jiraClient.AddComment(r.Context(), ticketKey, req.Comment)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "submitted"})
}

type CloseRequest struct {
	Resolution string `json:"resolution,omitempty"`
}

func (h *RFCHandler) closeRFC(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.jiraClient == nil {
		http.Error(w, "Jira integration not configured", http.StatusServiceUnavailable)
		return
	}

	ticketKey := chi.URLParam(r, "ticketKey")

	var req CloseRequest
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.jiraClient.CloseTicket(r.Context(), ticketKey, req.Resolution); err != nil {
		http.Error(w, "Failed to close ticket: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "closed"})
}

type DeployCommentRequest struct {
	User   string   `json:"user"`
	Hosts  []string `json:"hosts"`
	Result string   `json:"result"`
}

func (h *RFCHandler) addDeployComment(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.jiraClient == nil {
		http.Error(w, "Jira integration not configured", http.StatusServiceUnavailable)
		return
	}

	ticketKey := chi.URLParam(r, "ticketKey")

	var req DeployCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.jiraClient.AddDeployComment(r.Context(), ticketKey, jira.DeployInfo{
		User:   req.User,
		Hosts:  req.Hosts,
		Result: req.Result,
		Time:   time.Now(),
	}); err != nil {
		http.Error(w, "Failed to add deploy comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
