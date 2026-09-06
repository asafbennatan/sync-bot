package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type botRepo struct {
	DefaultBranch string `json:"default_branch"`
	Parent        *struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	} `json:"parent"`
}

type botURLInfo struct {
	Owner  string
	Repo   string
	Branch string
}

func parseBotURL(raw string) (botURLInfo, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return botURLInfo{}, fmt.Errorf("bot URL is empty")
	}

	if strings.HasPrefix(raw, "git@github.com:") {
		path := strings.TrimPrefix(raw, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return botURLInfo{}, fmt.Errorf("invalid git SSH URL: %s", raw)
		}
		return botURLInfo{Owner: parts[0], Repo: parts[1]}, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return botURLInfo{}, fmt.Errorf("invalid URL: %w", err)
	}

	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return botURLInfo{}, fmt.Errorf("expected github.com/owner/repo URL, got: %s", raw)
	}

	info := botURLInfo{Owner: parts[0], Repo: parts[1]}
	if len(parts) >= 4 && parts[2] == "tree" {
		info.Branch = parts[3]
	}
	return info, nil
}

func botGitURL(owner, repo string) string {
	return fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
}

func resolveBotURL(raw string) (Config, error) {
	info, err := parseBotURL(raw)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		BotRemoteURL: botGitURL(info.Owner, info.Repo),
		BotBranch:    info.Branch,
		TargetRemote: "origin",
	}

	repoJSON, err := runCmd("gh", "api", fmt.Sprintf("repos/%s/%s", info.Owner, info.Repo))
	if err != nil {
		return Config{}, fmt.Errorf("failed to query bot repo via gh: %w", err)
	}

	var repo botRepo
	if err := json.Unmarshal([]byte(repoJSON), &repo); err != nil {
		return Config{}, fmt.Errorf("failed to parse bot repo metadata: %w", err)
	}

	if cfg.BotBranch == "" {
		cfg.BotBranch, err = soleNonDefaultBranch(info.Owner, info.Repo, repo.DefaultBranch)
		if err != nil {
			return Config{}, err
		}
	}

	if repo.Parent != nil {
		cfg.BaseBranch = repo.Parent.DefaultBranch
		if cfg.BaseBranch == "" {
			parentJSON, err := runCmd("gh", "api", fmt.Sprintf("repos/%s", repo.Parent.FullName))
			if err != nil {
				return Config{}, fmt.Errorf("failed to query upstream repo via gh: %w", err)
			}
			var parent botRepo
			if err := json.Unmarshal([]byte(parentJSON), &parent); err != nil {
				return Config{}, fmt.Errorf("failed to parse upstream repo metadata: %w", err)
			}
			cfg.BaseBranch = parent.DefaultBranch
		}
	}
	if cfg.BaseBranch == "" {
		cfg.BaseBranch = repo.DefaultBranch
	}
	if cfg.BaseBranch == "" {
		cfg.BaseBranch = "main"
	}

	return cfg, nil
}

type branchRef struct {
	Name string `json:"name"`
}

func soleNonDefaultBranchFromList(branches []branchRef, defaultBranch string) (string, error) {
	var featureBranches []string
	for _, branch := range branches {
		if branch.Name != defaultBranch {
			featureBranches = append(featureBranches, branch.Name)
		}
	}

	switch len(featureBranches) {
	case 0:
		return "", fmt.Errorf("no feature branch found; include /tree/<branch> in the URL")
	case 1:
		return featureBranches[0], nil
	default:
		return "", fmt.Errorf(
			"multiple feature branches (%s); include /tree/<branch> in the URL",
			strings.Join(featureBranches, ", "),
		)
	}
}

func soleNonDefaultBranch(owner, repo, defaultBranch string) (string, error) {
	branchesJSON, err := runCmd("gh", "api", fmt.Sprintf("repos/%s/%s/branches", owner, repo))
	if err != nil {
		return "", fmt.Errorf("failed to list bot branches via gh: %w", err)
	}

	var branches []branchRef
	if err := json.Unmarshal([]byte(branchesJSON), &branches); err != nil {
		return "", fmt.Errorf("failed to parse branch list: %w", err)
	}

	branch, err := soleNonDefaultBranchFromList(branches, defaultBranch)
	if err != nil {
		return "", fmt.Errorf("on %s/%s: %w", owner, repo, err)
	}
	return branch, nil
}
