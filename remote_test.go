package main

import "testing"

func TestNormalizeGitHubURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{
			in:   "https://github.com/redhat-chai-bot/flightctl_flightctl.git",
			want: "github.com/redhat-chai-bot/flightctl_flightctl",
		},
		{
			in:   "git@github.com:flightctl/flightctl.git",
			want: "github.com/flightctl/flightctl",
		},
		{
			in:   "https://github.com/org/repo/",
			want: "github.com/org/repo",
		},
	}

	for _, tc := range tests {
		got := normalizeGitHubURL(tc.in)
		if got != tc.want {
			t.Fatalf("normalizeGitHubURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestOwnerRepoFromGitURL(t *testing.T) {
	owner, repo, err := ownerRepoFromGitURL("https://github.com/org/my-repo.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "org" || repo != "my-repo" {
		t.Fatalf("got %s/%s, want org/my-repo", owner, repo)
	}
}

func TestResolvePushRemoteExplicit(t *testing.T) {
	remote, err := resolvePushRemote(Config{
		TargetRemote:         "myfork",
		TargetRemoteExplicit: true,
	}, "upstream", false, stackBranch{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remote != "myfork" {
		t.Fatalf("got %q, want myfork", remote)
	}
}

func TestResolvePushRemoteUsesBaseRemote(t *testing.T) {
	remote, err := resolvePushRemote(Config{}, "upstream", false, stackBranch{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remote != "upstream" {
		t.Fatalf("got %q, want upstream", remote)
	}
}
