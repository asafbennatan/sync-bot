package main

import (
	"strings"
	"testing"
)

func TestFindStackLayer(t *testing.T) {
	view := stackView{
		Branches: []stackBranch{
			{Name: "layer-one", Head: "abc123"},
			{Name: "layer-two", Head: "def456", PR: &stackPR{Number: 42}},
		},
	}

	layer, ok := findStackLayer(view, "layer-two")
	if !ok {
		t.Fatal("expected to find layer-two")
	}
	if layer.Name != "layer-two" {
		t.Fatalf("got %q, want layer-two", layer.Name)
	}

	_, ok = findStackLayer(view, "main")
	if ok {
		t.Fatal("expected main not to be in stack")
	}
}

func TestDetectBotBaseBranchFromStackHead(t *testing.T) {
	view := stackView{
		Branches: []stackBranch{
			{Name: "EDM-5225-e2e-os-delta-hold", Head: "f082265e9"},
		},
	}

	layer, ok := findStackLayer(view, "EDM-5225-e2e-os-delta-hold")
	if !ok {
		t.Fatal("expected stack layer")
	}
	if layer.Name != "EDM-5225-e2e-os-delta-hold" {
		t.Fatalf("got %q", layer.Name)
	}
}


func TestRemoteRef(t *testing.T) {
	if remoteRef("upstream", "main") != "upstream/main" {
		t.Fatal("unexpected remote ref")
	}
}

func TestStackTop(t *testing.T) {
	view := stackView{
		Branches: []stackBranch{
			{Name: "layer-one"},
			{Name: "layer-two", PR: &stackPR{Number: 42}},
		},
	}

	top, err := stackTop(view)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if top.Name != "layer-two" {
		t.Fatalf("got %q, want layer-two", top.Name)
	}
	if stackLinkRef(top) != "42" {
		t.Fatalf("got link ref %q, want 42", stackLinkRef(top))
	}
}

func TestStackTopEmpty(t *testing.T) {
	_, err := stackTop(stackView{})
	if err == nil {
		t.Fatal("expected error for empty stack")
	}
}

func TestStackLinkRefWithoutPR(t *testing.T) {
	ref := stackLinkRef(stackBranch{Name: "feature"})
	if ref != "feature" {
		t.Fatalf("got %q, want feature", ref)
	}
}

func TestRepoFromPRURL(t *testing.T) {
	owner, repo, err := repoFromPRURL("https://github.com/flightctl/flightctl/pull/3464")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "flightctl" || repo != "flightctl" {
		t.Fatalf("got %s/%s, want flightctl/flightctl", owner, repo)
	}
}

func TestResolveBaseRemoteFromParent(t *testing.T) {
	cfg := Config{
		BaseRepo:     "flightctl/flightctl",
		TargetRemote: "origin",
	}
	_, err := resolveBaseRemote(cfg)
	if err == nil {
		t.Skip("git repo with upstream remote not available in test environment")
	}
	if !strings.Contains(err.Error(), "no git remote found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveBaseRemoteRequiresParentRepo(t *testing.T) {
	_, err := resolveBaseRemote(Config{TargetRemote: "origin"})
	if err == nil {
		t.Fatal("expected error when parent repo is unknown")
	}
}
