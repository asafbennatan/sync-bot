# sync-bot

Sync a [Chai Bot](https://github.com/redhat-chai-bot) branch into your local repository, rewrite commit authors to your identity, and push to your remote.

## Prerequisites

- [Go](https://go.dev/) 1.25+
- [git](https://git-scm.com/)
- [GitHub CLI](https://cli.github.com/) (`gh`) — required when using a bot URL (auto-detects branch and upstream)
- [gh-stack](https://github.com/nicholasjackson/gh-stack) — only needed for `--stack` mode

## Install

```bash
go install github.com/abennata/chai-cloner@latest
```

Or build from source:

```bash
git clone https://github.com/abennata/sync-bot.git
cd sync-bot
go build -o sync-bot .
```

## Usage

Run from inside the git repository you want to push to (your fork or origin).

### Quick start — paste the bot URL

```bash
sync-bot https://github.com/redhat-chai-bot/my-repo/tree/my-feature-branch
```

The tool will:

1. Add a `chai-bot-remote` git remote pointing at the bot repo
2. Fetch the bot branch and check it out locally
3. Rebase onto the upstream base branch and rewrite every commit author to your local `git config user.name` / `user.email`
4. Force-push (with lease) to `origin`

When you pass a GitHub URL, `gh` resolves the bot remote, branch name, and base branch automatically. If the URL has no `/tree/<branch>` segment and the bot repo has exactly one non-default branch, that branch is picked. Otherwise include the branch in the URL.

### Manual flags

Use flags instead of (or to override) URL resolution:

```bash
sync-bot \
  --bot-remote https://github.com/redhat-chai-bot/my-repo.git \
  --bot-branch my-feature-branch \
  --my-branch my-feature-branch \
  --target-remote origin \
  --base main
```

### Stacked PRs

If you use [gh stack](https://github.com/nicholasjackson/gh-stack) for stacked pull requests:

```bash
sync-bot --stack https://github.com/redhat-chai-bot/my-repo/tree/my-feature-branch
```

This runs `gh stack init` and `gh stack submit --auto` after the push.

## Options

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--url` | | | Bot GitHub repo or branch URL (alternative to positional arg) |
| `--bot-branch` | `-b` | | Branch created by Chai Bot |
| `--my-branch` | `-m` | same as bot branch | Local branch name to check out and push |
| `--bot-remote` | `-r` | | Git SSH/HTTPS URL for the bot repository |
| `--target-remote` | `-t` | `origin` | Remote to push the rewritten branch to |
| `--base` | `-a` | auto / `main` | Base branch to rebase against |
| `--stack` | `-s` | `false` | Enable gh stack integration |

Either provide a bot URL (positional or `--url`) or both `--bot-remote` and `--bot-branch`.

## What it does

```
Bot repo (chai-bot-remote)          Your repo (origin)
        │                                    │
        ├─ fetch bot branch                  │
        ├─ checkout as local branch          │
        ├─ rebase onto base branch           │
        ├─ amend each commit (--reset-author)│
        └─ push ────────────────────────────►│
```

Commit authorship is reset on every commit in the rebased range so the history shows you as the author, not the bot.

## Examples

```bash
# HTTPS URL with branch
sync-bot https://github.com/redhat-chai-bot/flightctl_vm-to-quadlet/tree/edm-5571-stop-timeout-podmanargs

# SSH URL (branch must be specified separately or auto-detected)
sync-bot git@github.com:redhat-chai-bot/flightctl_vm-to-quadlet.git --bot-branch edm-5571-stop-timeout-podmanargs

# Push to a differently named local branch
sync-bot --url https://github.com/redhat-chai-bot/my-repo/tree/bot-branch -m my-local-branch
```
