package main

import (
	"fmt"
	"os"
)

func ensureCleanGitState() {
	if gitStatePresent("rebase-merge") || gitStatePresent("rebase-apply") {
		fmt.Println("Aborting in-progress rebase...")
		runCmd("git", "rebase", "--abort")
	}
	if gitStatePresent("MERGE_HEAD") {
		fmt.Println("Aborting in-progress merge...")
		runCmd("git", "merge", "--abort")
	}
	if gitStatePresent("CHERRY_PICK_HEAD") {
		fmt.Println("Aborting in-progress cherry-pick...")
		runCmd("git", "cherry-pick", "--abort")
	}
}

func gitStatePresent(name string) bool {
	path, err := runCmd("git", "rev-parse", "--git-path", name)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
