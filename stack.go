package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type stackPR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type stackBranch struct {
	Name     string   `json:"name"`
	Head     string   `json:"head"`
	IsMerged bool     `json:"isMerged"`
	PR       *stackPR `json:"pr"`
}

type stackView struct {
	Branches []stackBranch `json:"branches"`
}

func loadStackView() (stackView, error) {
	out, err := runCmd("gh", "stack", "view", "--json")
	if err != nil {
		return stackView{}, fmt.Errorf("failed to read gh stack: %w", err)
	}

	var view stackView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		return stackView{}, fmt.Errorf("failed to parse gh stack view: %w", err)
	}

	return view, nil
}

func loadStackViewOptional(baseRemote, botRef, defaultBranch string) stackView {
	view, err := loadStackView()
	if err == nil && len(view.Branches) > 0 {
		return view
	}

	stackBranch := inferStackBaseBranch(baseRemote, botRef, defaultBranch)
	if stackBranch == "" || stackBranch == defaultBranch {
		return stackView{}
	}

	return loadStackViewFromBranch(stackBranch)
}

func loadStackViewFromBranch(branch string) stackView {
	if branch == "" {
		return stackView{}
	}

	originalRef, err := runCmd("git", "rev-parse", "HEAD")
	if err != nil {
		return stackView{}
	}
	originalBranch, _ := runCmd("git", "rev-parse", "--abbrev-ref", "HEAD")

	if _, err := runCmd("git", "checkout", branch); err != nil {
		restoreGitHEAD(originalRef, originalBranch)
		return stackView{}
	}

	view, err := loadStackView()
	restoreGitHEAD(originalRef, originalBranch)

	if err != nil {
		return stackView{}
	}
	return view
}

func restoreGitHEAD(ref, branch string) {
	if branch != "" && branch != "HEAD" {
		runCmd("git", "checkout", branch)
		return
	}
	runCmd("git", "checkout", ref)
}

func findStackLayer(view stackView, branchName string) (stackBranch, bool) {
	for _, branch := range view.Branches {
		if branch.Name == branchName {
			return branch, true
		}
	}
	return stackBranch{}, false
}

func loadStackTop() (stackBranch, error) {
	view, err := loadStackView()
	if err != nil {
		return stackBranch{}, err
	}
	return stackTop(view)
}

func stackTop(view stackView) (stackBranch, error) {
	if len(view.Branches) == 0 {
		return stackBranch{}, fmt.Errorf("no branches in current gh stack")
	}
	return view.Branches[len(view.Branches)-1], nil
}

func stackLinkRef(branch stackBranch) string {
	if branch.PR != nil && branch.PR.Number > 0 {
		return fmt.Sprintf("%d", branch.PR.Number)
	}
	return branch.Name
}

func adoptAndSubmitStack(targetRemote, stackLayerBranch, newBranch string) error {
	if stackLayerBranch == "" || newBranch == "" {
		return fmt.Errorf("stack layer and branch name are required")
	}

	originalBranch, _ := runCmd("git", "rev-parse", "--abbrev-ref", "HEAD")

	if _, err := runCmd("git", "checkout", stackLayerBranch); err != nil {
		return fmt.Errorf("failed to checkout stack layer %q: %w", stackLayerBranch, err)
	}

	view, err := loadStackView()
	alreadyTracked := err == nil
	if alreadyTracked {
		_, alreadyTracked = findStackLayer(view, newBranch)
	}
	if !alreadyTracked {
		if _, err := runCmd("gh", "stack", "add", newBranch); err != nil {
			restoreGitHEAD("", originalBranch)
			return fmt.Errorf("failed to add branch to gh stack: %w", err)
		}
		fmt.Printf("Added '%s' to local gh stack\n", newBranch)
	}

	if _, err := runCmd("git", "checkout", newBranch); err != nil {
		restoreGitHEAD("", originalBranch)
		return fmt.Errorf("failed to checkout %q: %w", newBranch, err)
	}

	if _, err := runCmd("gh", "stack", "submit", "--auto", "--remote", targetRemote); err != nil {
		return fmt.Errorf("failed to submit gh stack: %w", err)
	}

	return nil
}

func resolveRebaseTarget(remote, branch string) (string, error) {
	if _, err := runCmd("git", "fetch", remote, branch); err != nil {
		return "", fmt.Errorf("failed to fetch %q from %q: %w", branch, remote, err)
	}

	return remoteRef(remote, branch), nil
}

func remoteRef(remote, branch string) string {
	return fmt.Sprintf("%s/%s", remote, branch)
}

func resolveStackTargetRemote(stackTop stackBranch, fallbackRemote string, explicit bool) (string, error) {
	if explicit {
		return fallbackRemote, nil
	}

	if stackTop.PR == nil || stackTop.PR.URL == "" {
		return fallbackRemote, nil
	}

	owner, repo, err := repoFromPRURL(stackTop.PR.URL)
	if err != nil {
		return "", err
	}

	remote, err := gitRemoteForRepo(owner, repo)
	if err != nil {
		return "", err
	}

	return remote, nil
}

func repoFromPRURL(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid stack PR URL %q: %w", raw, err)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid stack PR URL %q", raw)
	}

	return parts[0], parts[1], nil
}

func gitRemoteForRepo(owner, repo string) (string, error) {
	out, err := runCmd("git", "remote", "-v")
	if err != nil {
		return "", fmt.Errorf("failed to list git remotes: %w", err)
	}

	want := fmt.Sprintf("%s/%s", owner, repo)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] != "(fetch)" {
			continue
		}
		if strings.Contains(fields[1], want) {
			return fields[0], nil
		}
	}

	return "", fmt.Errorf("no git remote found for %s/%s; add a git remote for the parent repo or pass --target-remote", owner, repo)
}
