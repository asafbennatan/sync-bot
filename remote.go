package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const defaultBotRemoteName = "chai-bot-remote"

func ensureBotRemote(botRemoteURL string) (string, error) {
	if existing := findRemoteByURL(botRemoteURL); existing != "" {
		return existing, nil
	}

	remotes, _ := runCmd("git", "remote")
	if strings.Contains(remotes, defaultBotRemoteName) {
		currentURL, err := runCmd("git", "remote", "get-url", defaultBotRemoteName)
		if err == nil && normalizeGitHubURL(currentURL) == normalizeGitHubURL(botRemoteURL) {
			return defaultBotRemoteName, nil
		}
	}

	fmt.Printf("Adding remote '%s'...\n", defaultBotRemoteName)
	if _, err := runCmd("git", "remote", "add", defaultBotRemoteName, botRemoteURL); err != nil {
		return "", fmt.Errorf("failed to add bot remote: %w", err)
	}

	return defaultBotRemoteName, nil
}

func findRemoteByURL(wantURL string) string {
	want := normalizeGitHubURL(wantURL)
	out, err := runCmd("git", "remote", "-v")
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] != "(fetch)" {
			continue
		}
		if normalizeGitHubURL(fields[1]) == want {
			return fields[0]
		}
	}

	return ""
}

func normalizeGitHubURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, ".git")

	if strings.HasPrefix(raw, "git@github.com:") {
		path := strings.TrimPrefix(raw, "git@github.com:")
		return "github.com/" + strings.ToLower(path)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(raw)
	}

	return strings.ToLower(strings.TrimSuffix(u.Host+u.Path, "/"))
}

func resolveParentRepo(cfg *Config) error {
	if cfg.BaseRepo != "" {
		return nil
	}

	owner, repo, err := ownerRepoFromGitURL(cfg.BotRemoteURL)
	if err != nil {
		return err
	}

	repoJSON, err := runCmd("gh", "api", fmt.Sprintf("repos/%s/%s", owner, repo))
	if err != nil {
		return fmt.Errorf("failed to query bot repo via gh: %w", err)
	}

	var botRepoMeta botRepo
	if err := json.Unmarshal([]byte(repoJSON), &botRepoMeta); err != nil {
		return fmt.Errorf("failed to parse bot repo metadata: %w", err)
	}

	if botRepoMeta.Parent != nil && botRepoMeta.Parent.FullName != "" {
		cfg.BaseRepo = botRepoMeta.Parent.FullName
		if cfg.BaseBranch == "" {
			cfg.BaseBranch = botRepoMeta.Parent.DefaultBranch
		}
		return nil
	}

	viewJSON, err := runCmd("gh", "repo", "view", "--json", "nameWithOwner,defaultBranchRef,parent")
	if err != nil {
		return fmt.Errorf("failed to resolve parent repo for %s/%s: %w", owner, repo, err)
	}

	var view ghRepoView
	if err := json.Unmarshal([]byte(viewJSON), &view); err != nil {
		return fmt.Errorf("failed to parse current repo metadata: %w", err)
	}

	if view.Parent != nil && view.Parent.NameWithOwner != "" {
		cfg.BaseRepo = view.Parent.NameWithOwner
	} else if view.NameWithOwner != "" {
		cfg.BaseRepo = view.NameWithOwner
	}

	if cfg.BaseBranch == "" {
		cfg.BaseBranch = view.DefaultBranchName()
	}

	return nil
}

type ghRepoView struct {
	NameWithOwner string `json:"nameWithOwner"`
	DefaultBranch *struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
	Parent *struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"parent"`
}

func (v ghRepoView) DefaultBranchName() string {
	if v.DefaultBranch != nil && v.DefaultBranch.Name != "" {
		return v.DefaultBranch.Name
	}
	return ""
}

func ownerRepoFromGitURL(raw string) (string, string, error) {
	normalized := normalizeGitHubURL(raw)
	const prefix = "github.com/"
	if !strings.HasPrefix(normalized, prefix) {
		return "", "", fmt.Errorf("expected a GitHub repository URL, got: %s", raw)
	}

	path := strings.TrimPrefix(normalized, prefix)
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub repository URL: %s", raw)
	}

	return parts[0], parts[1], nil
}

func resolveBaseRemote(cfg Config) (string, error) {
	if cfg.BaseRepo == "" {
		return "", fmt.Errorf("parent repository is unknown; pass a bot URL or set --base")
	}

	parts := strings.Split(cfg.BaseRepo, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid parent repo %q", cfg.BaseRepo)
	}

	return gitRemoteForRepo(parts[0], parts[1])
}

func resolvePushRemote(cfg Config, baseRemote string, onStack bool, stackLayer stackBranch) (string, error) {
	if cfg.TargetRemoteExplicit {
		if cfg.TargetRemote == "" {
			return "", fmt.Errorf("pass --target-remote when overriding the push remote")
		}
		return cfg.TargetRemote, nil
	}

	if onStack {
		return resolveStackTargetRemote(stackLayer, baseRemote, false)
	}

	if baseRemote != "" {
		return baseRemote, nil
	}

	if cfg.TargetRemote != "" {
		return cfg.TargetRemote, nil
	}

	pushDefault, err := runCmd("git", "config", "--get", "remote.pushDefault")
	if err == nil && pushDefault != "" {
		return pushDefault, nil
	}

	return "", fmt.Errorf("could not determine push remote; pass --target-remote")
}
