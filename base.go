package main

import (
	"fmt"
	"regexp"
	"strings"
)

var branchTicketRE = regexp.MustCompile(`^([A-Z]+-\d+)`)
var commitTicketRE = regexp.MustCompile(`^([A-Z]+-\d+):`)

func detectBotBase(baseRemote, defaultBranch, botRef string, stackView stackView) (string, string, error) {
	botTipParent, err := runCmd("git", "rev-parse", botRef+"^")
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve bot branch parent: %w", err)
	}

	newCommitCount, err := runCmd("git", "rev-list", "--count", botTipParent+".."+botRef)
	if err != nil {
		return "", "", fmt.Errorf("failed to count bot commits: %w", err)
	}
	if newCommitCount == "0" {
		return "", "", fmt.Errorf("bot branch has no commits to sync")
	}

	mainRef, err := resolveRebaseTarget(baseRemote, defaultBranch)
	if err != nil {
		return "", "", err
	}

	mainOnlyCount, err := runCmd("git", "rev-list", "--count", mainRef+".."+botRef)
	if err != nil {
		return "", "", err
	}

	if newCommitCount == mainOnlyCount {
		return defaultBranch, botTipParent, nil
	}

	parentSubject, err := runCmd("git", "log", "-1", "--format=%s", botTipParent)
	if err != nil {
		return "", "", err
	}
	parentTicket := ticketFromCommit(parentSubject)

	for i := len(stackView.Branches) - 1; i >= 0; i-- {
		layer := stackView.Branches[i]
		if parentTicket == "" || parentTicket != ticketFromBranch(layer.Name) {
			continue
		}
		return layer.Name, botTipParent, nil
	}

	forkPoint, err := botForkPointForLayer(mainRef, botRef)
	if err != nil {
		return "", "", err
	}

	return defaultBranch, forkPoint, nil
}

func botForkPointForLayer(layerRef, botRef string) (string, error) {
	firstNew, err := runCmd("git", "rev-list", "--reverse", layerRef+".."+botRef)
	if err != nil {
		return "", fmt.Errorf("failed to list bot commits on top of %q: %w", layerRef, err)
	}

	lines := strings.Split(strings.TrimSpace(firstNew), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("no commits on bot branch above %q", layerRef)
	}

	forkPoint, err := runCmd("git", "rev-parse", lines[0]+"^")
	if err != nil {
		return "", fmt.Errorf("failed to resolve fork point for bot branch: %w", err)
	}

	return forkPoint, nil
}

func ticketFromBranch(name string) string {
	match := branchTicketRE.FindStringSubmatch(name)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func ticketFromCommit(subject string) string {
	match := commitTicketRE.FindStringSubmatch(subject)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func rebaseBotOntoBase(baseRemote, defaultBranch, botRef, rebaseExec string, stackView stackView) (string, string, error) {
	baseBranch, forkPoint, err := detectBotBase(baseRemote, defaultBranch, botRef, stackView)
	if err != nil {
		return "", "", err
	}

	onto, err := resolveRebaseTarget(baseRemote, baseBranch)
	if err != nil {
		return "", "", err
	}

	if _, err := runCmd("git", "rebase", "--onto", onto, forkPoint, "--exec", rebaseExec); err != nil {
		return "", "", fmt.Errorf("failed to rewrite commit authors during rebase: %w", err)
	}

	return baseBranch, forkPoint, nil
}
