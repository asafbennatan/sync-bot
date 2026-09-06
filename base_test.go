package main

import "testing"

func TestTicketFromBranch(t *testing.T) {
	if ticketFromBranch("EDM-5225-e2e-os-delta-hold") != "EDM-5225" {
		t.Fatal("unexpected ticket from branch")
	}
}

func TestTicketFromCommit(t *testing.T) {
	if ticketFromCommit("EDM-5225: Stall on pair counts") != "EDM-5225" {
		t.Fatal("unexpected ticket from commit")
	}
}
