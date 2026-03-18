package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v60/github"
)

type GitHubClients struct {
	appID          int64
	privateKeyPath string
	mu             sync.Mutex
	clients        map[int64]*github.Client
}

func NewGitHubClients(appID int64, privateKeyPath string) *GitHubClients {
	return &GitHubClients{
		appID:          appID,
		privateKeyPath: privateKeyPath,
		clients:        make(map[int64]*github.Client),
	}
}

func (gc *GitHubClients) GetClient(installationID int64) (*github.Client, error) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	if client, ok := gc.clients[installationID]; ok {
		return client, nil
	}

	transport, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, gc.appID, installationID, gc.privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("creating installation transport: %w", err)
	}

	client := github.NewClient(&http.Client{Transport: transport})
	gc.clients[installationID] = client
	return client, nil
}

func LockRepo(ctx context.Context, client *github.Client, owner, repo, username string) error {
	opts := &github.RepositoryAddCollaboratorOptions{
		Permission: "pull",
	}
	_, _, err := client.Repositories.AddCollaborator(ctx, owner, repo, username, opts)
	if err != nil {
		return fmt.Errorf("setting %s to read-only on %s/%s: %w", username, owner, repo, err)
	}
	log.Printf("Locked repo %s/%s for user %s (set to read-only)", owner, repo, username)
	return nil
}

func UnlockRepo(ctx context.Context, client *github.Client, owner, repo, username string) error {
	opts := &github.RepositoryAddCollaboratorOptions{
		Permission: "push",
	}
	_, _, err := client.Repositories.AddCollaborator(ctx, owner, repo, username, opts)
	if err != nil {
		return fmt.Errorf("restoring write access for %s on %s/%s: %w", username, owner, repo, err)
	}
	log.Printf("Unlocked repo %s/%s for user %s (restored write access)", owner, repo, username)
	return nil
}

func findStudent(ctx context.Context, client *github.Client, owner, repo string) (string, error) {
	collaborators, _, err := client.Repositories.ListCollaborators(ctx, owner, repo, &github.ListCollaboratorsOptions{
		Affiliation: "outside",
	})
	if err != nil {
		return "", fmt.Errorf("listing collaborators: %w", err)
	}

	for _, c := range collaborators {
		login := c.GetLogin()
		if !strings.HasSuffix(login, "[bot]") {
			return login, nil
		}
	}

	if idx := strings.LastIndex(repo, "-"); idx != -1 && idx < len(repo)-1 {
		username := repo[idx+1:]
		log.Printf("No outside collaborator found, falling back to repo name: %s", username)
		return username, nil
	}

	return "", fmt.Errorf("no student collaborator found on %s/%s", owner, repo)
}

func HasWriteAccess(ctx context.Context, client *github.Client, owner, repo, username string) bool {
	perm, _, err := client.Repositories.GetPermissionLevel(ctx, owner, repo, username)
	if err != nil {
		log.Printf("ERROR: checking permission for %s on %s/%s: %v", username, owner, repo, err)
		return false
	}
	level := perm.GetPermission()
	return level == "write" || level == "admin"
}

func ReAddReviewer(ctx context.Context, client *github.Client, owner, repo string, prNumber int, login string) error {
	_, _, err := client.PullRequests.RequestReviewers(ctx, owner, repo, prNumber, github.ReviewersRequest{
		Reviewers: []string{login},
	})
	if err != nil {
		return fmt.Errorf("re-adding reviewer %s to %s/%s#%d: %w", login, owner, repo, prNumber, err)
	}
	log.Printf("Re-added reviewer %s to %s/%s#%d", login, owner, repo, prNumber)
	return nil
}

func RemoveReviewer(ctx context.Context, client *github.Client, owner, repo string, prNumber int, login string) error {
	_, err := client.PullRequests.RemoveReviewers(ctx, owner, repo, prNumber, github.ReviewersRequest{
		Reviewers: []string{login},
	})
	if err != nil {
		return fmt.Errorf("removing reviewer %s from %s/%s#%d: %w", login, owner, repo, prNumber, err)
	}
	log.Printf("Removed reviewer %s from %s/%s#%d", login, owner, repo, prNumber)
	return nil
}
