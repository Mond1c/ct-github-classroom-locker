package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
)

type WebhookHandler struct {
	ghClients      *GitHubClients
	sheetsClient   *SheetsClient
	webhookSecret  []byte
	reviewTeamSlug string
	reviewerLogin  string
}

func NewWebhookHandler(ghClients *GitHubClients, sheetsClient *SheetsClient, webhookSecret string, reviewTeamSlug string, reviewerLogin string) *WebhookHandler {
	return &WebhookHandler{
		ghClients:      ghClients,
		sheetsClient:   sheetsClient,
		webhookSecret:  []byte(webhookSecret),
		reviewTeamSlug: reviewTeamSlug,
		reviewerLogin:  reviewerLogin,
	}
}

func (wh *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, wh.webhookSecret)
	if err != nil {
		log.Printf("Invalid webhook signature: %v", err)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	eventType := github.WebHookType(r)
	event, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		log.Printf("Failed to parse webhook: %v", err)
		http.Error(w, "failed to parse webhook", http.StatusBadRequest)
		return
	}

	switch e := event.(type) {
	case *github.PullRequestEvent:
		wh.handlePullRequestEvent(r.Context(), e)
	case *github.PullRequestReviewEvent:
		wh.handlePullRequestReviewEvent(r.Context(), e)
	default:
		log.Printf("Ignoring event type: %s", eventType)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func (wh *WebhookHandler) handlePullRequestEvent(ctx context.Context, event *github.PullRequestEvent) {
	action := event.GetAction()

	switch action {
	case "review_requested":
		wh.handleReviewRequested(ctx, event)
	case "review_request_removed":
		wh.handleReviewRequestRemoved(ctx, event)
	default:
		log.Printf("Ignoring pull_request action: %s", action)
	}
}

func isDigitsOnly(s string) bool {
        if len(s) == 0 {
                return false
        }
        for _, r := range s {
                if r < '0' || r > '9' {
                        return false
                }
        }
        return true
}

func (wh *WebhookHandler) handleReviewRequested(ctx context.Context, event *github.PullRequestEvent) {
	reviewer := event.GetRequestedReviewer()
	if reviewer == nil || !strings.EqualFold(reviewer.GetLogin(), wh.reviewerLogin) {
		log.Printf("Review requested for non-target reviewer, ignoring")
		return
	}

	sender := event.GetSender().GetLogin()

	if strings.HasSuffix(sender, "[bot]") {
		log.Printf("Ignoring review_requested from bot: %s", sender)
		return
	}

	installationID := event.GetInstallation().GetID()
	owner := event.GetRepo().GetOwner().GetLogin()
	repo := event.GetRepo().GetName()
	prURL := event.GetPullRequest().GetHTMLURL()
	repoURL := event.GetRepo().GetHTMLURL()

	log.Printf("Review requested by %s for %s on %s/%s", sender, wh.reviewerLogin, owner, repo)

	client, err := wh.ghClients.GetClient(installationID)
	if err != nil {
		log.Printf("ERROR: getting GitHub client: %v", err)
		return
	}

	if err := LockRepo(ctx, client, owner, repo, sender); err != nil {
		log.Printf("ERROR: locking repo: %v", err)
	}

	if isDigitsOnly(sender) && sender[0] == '0' {
                sender = fmt.Sprintf("'%s", sender)
        }

	msk := time.FixedZone("MSK", 3*60*60)
	timestamp := time.Now().In(msk).Format("2006-01-02 15:04:05")
	if err := wh.sheetsClient.AppendRow(ctx, []interface{}{timestamp, sender, repoURL, prURL}); err != nil {
		log.Printf("ERROR: appending to Google Sheets: %v", err)
	}
}

func (wh *WebhookHandler) handlePullRequestReviewEvent(ctx context.Context, event *github.PullRequestReviewEvent) {
	if event.GetAction() != "submitted" {
		return
	}

	state := event.GetReview().GetState()
	if state != "approved" && state != "changes_requested" {
		return
	}

	installationID := event.GetInstallation().GetID()
	org := event.GetRepo().GetOwner().GetLogin()
	repo := event.GetRepo().GetName()
	reviewer := event.GetReview().GetUser().GetLogin()

	client, err := wh.ghClients.GetClient(installationID)
	if err != nil {
		log.Printf("ERROR: getting GitHub client: %v", err)
		return
	}

	_, resp, err := client.Teams.GetTeamMembershipBySlug(ctx, org, wh.reviewTeamSlug, reviewer)
	if err != nil || resp.StatusCode == 404 {
		log.Printf("Reviewer %s is not a member of team %s, ignoring", reviewer, wh.reviewTeamSlug)
		return
	}

	student, err := findStudent(ctx, client, org, repo)
	if err != nil {
		log.Printf("ERROR: finding student for %s/%s: %v", org, repo, err)
		return
	}

	log.Printf("Review %s by %s (team %s) on %s/%s — restoring write access for %s",
		state, reviewer, wh.reviewTeamSlug, org, repo, student)

	if err := UnlockRepo(ctx, client, org, repo, student); err != nil {
		log.Printf("ERROR: unlocking repo: %v", err)
	}

	prNumber := event.GetPullRequest().GetNumber()
	if err := RemoveReviewer(ctx, client, org, repo, prNumber, wh.reviewerLogin); err != nil {
		log.Printf("ERROR: removing reviewer: %v", err)
	}
}

func (wh *WebhookHandler) handleReviewRequestRemoved(ctx context.Context, event *github.PullRequestEvent) {
	reviewer := event.GetRequestedReviewer()
	if reviewer == nil || !strings.EqualFold(reviewer.GetLogin(), wh.reviewerLogin) {
		return
	}

	sender := event.GetSender().GetLogin()
	if strings.EqualFold(sender, wh.reviewerLogin) || strings.HasSuffix(sender, "[bot]") {
		log.Printf("Allowing %s to remove reviewer %s (self or bot)", sender, wh.reviewerLogin)
		return
	}

	installationID := event.GetInstallation().GetID()
	owner := event.GetRepo().GetOwner().GetLogin()
	repo := event.GetRepo().GetName()
	prNumber := event.GetPullRequest().GetNumber()

	client, err := wh.ghClients.GetClient(installationID)
	if err != nil {
		log.Printf("ERROR: getting GitHub client: %v", err)
		return
	}

	if HasWriteAccess(ctx, client, owner, repo, sender) {
		log.Printf("Allowing %s to remove reviewer %s from %s/%s#%d (has write access)", sender, wh.reviewerLogin, owner, repo, prNumber)
		return
	}

	log.Printf("WARNING: %s attempted to remove reviewer %s from %s/%s#%d (no write access)", sender, wh.reviewerLogin, owner, repo, prNumber)

	if err := ReAddReviewer(ctx, client, owner, repo, prNumber, wh.reviewerLogin); err != nil {
		log.Printf("ERROR: re-adding reviewer: %v", err)
	}
}
