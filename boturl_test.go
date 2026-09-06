package main

import "testing"

func TestParseBotURL(t *testing.T) {
	tests := []struct {
		in      string
		owner   string
		repo    string
		branch  string
		wantErr bool
	}{
		{
			in:     "https://github.com/redhat-chai-bot/flightctl_vm-to-quadlet/tree/edm-5571-stop-timeout-podmanargs",
			owner:  "redhat-chai-bot",
			repo:   "flightctl_vm-to-quadlet",
			branch: "edm-5571-stop-timeout-podmanargs",
		},
		{
			in:    "https://github.com/redhat-chai-bot/flightctl_vm-to-quadlet",
			owner: "redhat-chai-bot",
			repo:  "flightctl_vm-to-quadlet",
		},
		{
			in:     "git@github.com:redhat-chai-bot/flightctl_vm-to-quadlet.git",
			owner:  "redhat-chai-bot",
			repo:   "flightctl_vm-to-quadlet",
			branch: "",
		},
	}

	for _, tc := range tests {
		info, err := parseBotURL(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("parseBotURL(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseBotURL(%q): %v", tc.in, err)
		}
		if info.Owner != tc.owner || info.Repo != tc.repo || info.Branch != tc.branch {
			t.Fatalf("parseBotURL(%q) = %+v, want owner=%q repo=%q branch=%q", tc.in, info, tc.owner, tc.repo, tc.branch)
		}
	}
}

func TestSoleNonDefaultBranch(t *testing.T) {
	branches := []branchRef{
		{Name: "master"},
		{Name: "feature-a"},
	}

	name, err := soleNonDefaultBranchFromList(branches, "master")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "feature-a" {
		t.Fatalf("got branch %q, want feature-a", name)
	}
}
