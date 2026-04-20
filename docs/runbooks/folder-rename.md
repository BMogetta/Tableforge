# Runbook: Rename local folder `tableforge/` → `recess/`

## Why this exists

The repository was renamed on GitHub from `tableforge` to `recess` on
2026-04-01 (commit `a9cfab0`). The local clone kept the original folder
name for continuity. At the scale of homelab iteration that didn't matter,
but it shows up as noise any time a path is printed (Claude Code memory
paths, VS Code window titles, shell history, tmux pane titles).

This runbook renames the clone cleanly **without losing any local state**:
git history, Claude Code memories, VS Code workspace, installed plugins,
and shell-pinned directories all survive.

## What gets renamed

| Path | Before | After |
| --- | --- | --- |
| Repo clone | `~/Github/tableforge/` | `~/Github/recess/` |
| Claude Code memory dir | `~/.claude/projects/-home-bruno-Github-tableforge/` | `~/.claude/projects/-home-bruno-Github-recess/` |
| Claude plugin project-scope paths | `~/.claude/plugins/installed_plugins.json` (entries with `projectPath: /home/bruno/Github/tableforge`) | same field, pointing at `recess` |

## What does NOT need to change

- `origin` git remote — already `git@...:BMogetta/recess.git` since 2026-04-01.
- `~/.kube/config-recess` — already named correctly.
- `CLAUDE.md`, `MEMORY.md`, any `*.md` inside the memory dir — these are
  content, not paths. They move with the dir.
- Anything inside the repo (the repo has no hardcoded
  `/home/bruno/Github/tableforge` references; verified with
  `grep -r /home/bruno/Github/tableforge .`).

## Prerequisites

- Close this Claude Code session (or any session rooted at the old path).
  You cannot rename a directory that has live file handles open inside.
- Close VS Code if it has the folder open. Re-open it after the rename.
- Stop any `kubectl port-forward`, `gh run watch`, or long-running
  `claude` processes running from the old path.

## Procedure

Run each step; none of them rolls back cleanly once the rename happens,
so only start when you're at a convenient stopping point.

```bash
# 1. Move the repo clone.
mv ~/Github/tableforge ~/Github/recess

# 2. Move the Claude Code memory + session transcripts so the next
#    claude session finds them at the new slug.
mv ~/.claude/projects/-home-bruno-Github-tableforge \
   ~/.claude/projects/-home-bruno-Github-recess

# 3. Patch installed_plugins.json so project-scoped plugins
#    (frontend-design, playwright) resolve at the new path.
sed -i 's|/home/bruno/Github/tableforge|/home/bruno/Github/recess|g' \
    ~/.claude/plugins/installed_plugins.json

# 4. Re-open VS Code at the new path.
code ~/Github/recess

# 5. Start a fresh Claude session.
cd ~/Github/recess && claude
```

## Validation

After the rename, from the new clone:

```bash
# Git remote is unchanged (still recess.git, expected).
git remote -v

# Memory index is present and readable.
cat ~/.claude/projects/-home-bruno-Github-recess/memory/MEMORY.md | head

# No stale plugin paths.
grep tableforge ~/.claude/plugins/installed_plugins.json && echo "STALE" || echo "OK"

# No broken symlinks anywhere in $HOME pointing at the old path.
find ~ -type l -lname '*Github/tableforge*' 2>/dev/null
```

All four checks should come back clean.

## Known caveats

- **Shell history**: commands that had the old path baked in (`cd
  ~/Github/tableforge`, editor invocations, etc.) will fail when
  replayed from reverse-search. Not destructive — you'll just have to
  re-type the path. The hit is cosmetic.

- **VS Code workspace files** (`*.code-workspace`): if you have a saved
  workspace file that lists `/home/bruno/Github/tableforge` as a folder,
  VS Code will show it as missing when you open the workspace. Either
  edit the `folders[].path` entry in the file or delete + re-create the
  workspace pointing at the new path.

- **Old Claude Code session transcripts** under the moved memory dir
  still contain the literal string `/home/bruno/Github/tableforge` in
  their chat logs. That's fine — they're historical records of what was
  true at the time; don't rewrite them.

- **GitHub CLI cached auth**: no-op. `gh` stores credentials under
  `~/.config/gh/`, keyed by host not by path.

## Rollback

`mv` is reversible. If anything goes wrong:

```bash
mv ~/Github/recess ~/Github/tableforge
mv ~/.claude/projects/-home-bruno-Github-recess \
   ~/.claude/projects/-home-bruno-Github-tableforge
sed -i 's|/home/bruno/Github/recess|/home/bruno/Github/tableforge|g' \
    ~/.claude/plugins/installed_plugins.json
```
