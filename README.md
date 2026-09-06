# sync-bot

Sync a [Chai Bot](https://github.com/redhat-chai-bot) branch into your local repository, rewrite commit authors to your identity, and push to the correct remote.

## Prerequisites

- [Go](https://go.dev/) 1.25+
- [git](https://git-scm.com/)
- [GitHub CLI](https://cli.github.com/) (`gh`) — resolves bot branch, parent repo, and remotes
- [gh-stack](https://github.com/github/gh-stack) — only when the bot branch extends an existing gh stack

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

Run from inside the target git repository. The tool discovers everything from the bot fork metadata and your configured git remotes — no repo-specific setup required.

```bash
sync-bot https://github.com/redhat-chai-bot/<repo>/tree/<branch>
```

### What it does

1. Reuses an existing git remote for the bot repo when one is already configured, otherwise adds `chai-bot-remote`
2. Resolves the bot fork's **parent repository** and default branch via `gh`
3. Maps the parent repo to whichever local git remote points at it (`upstream`, `origin`, etc.)
4. Detects which branch the bot forked from (parent default branch or a gh stack layer)
5. Replays only the bot's commits onto that branch's current tip (using remote-tracking refs, never stale local branches)
6. Rewrites commit authors to your local git identity
7. Pushes to the parent repo's git remote (or the stack PR remote), unless `--target-remote` is set

### Stack vs single branch (auto-detected)

| Bot forked from | What happens |
|-----------------|--------------|
| Parent default branch | Rebases onto its current tip, pushes to the parent repo's git remote |
| A branch in your active gh stack | Rebases onto that layer's tip, adopts into the stack via `gh stack add`, submits via `gh stack submit` |

## Options

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--url` | | | Bot GitHub repo or branch URL (alternative to positional arg) |
| `--bot-branch` | `-b` | | Branch created by Chai Bot |
| `--my-branch` | `-m` | same as bot branch | Local branch name to check out and push |
| `--bot-remote` | `-r` | | Git SSH/HTTPS URL for the bot repository |
| `--target-remote` | `-t` | auto | Remote to push to (defaults to parent repo remote) |
| `--base` | `-a` | auto | Parent repo default branch for fork-point detection |

Either provide a bot URL (positional or `--url`) or both `--bot-remote` and `--bot-branch`.

## Requirements

Your local repo must have a git remote whose URL points at the bot fork's parent repository on GitHub. The remote name does not matter — it is matched by URL.

```bash
git remote -v
# upstream  https://github.com/org/project.git (fetch)
```

If you push to a fork instead, pass `--target-remote <your-fork-remote>`.
