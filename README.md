# mdstrack

`mds` is a narrow terminal dashboard for the repositories you committed to work on this week. It discovers local git repositories, tracks a weekly assignment, counts your non-merge commits against a fixed target of three, and stores state atomically as JSON.

## Install and run

```bash
go install ./cmd/mds
mds setup MDS ~/Documents/MDS you@student.ubc.ca "Your Name"
mds
```

To use an enterprise host such as UBC GitHub, pass it as the final setup argument:

```bash
mds setup MDS ~/Documents/MDS you@student.ubc.ca "Your Name" github.ubc.ca
```

Inside the dashboard, press `b` to browse discovered local repositories and
repositories available through the configured GitHub host. Use `a` to add a
repository to the current week, `d` to remove it, `Space` to mark it done,
`c` to clone a remote repository, and `Enter` to open a local working copy.

The dashboard is intentionally read-only with respect to git history: it never stages, commits, rebases, or modifies a repository.

## Current commands

```text
mds status [--json]
mds scan [--json]
mds add|remove|done <repo>
mds week name <name>
mds week due <YYYY-MM-DD [HH:MM]|none>
mds workspace list|use <name>
mds auth [--json]
```

State is written atomically to the platform config directory, normally `~/.config/mdstrack/state.json` on Linux.

## Architecture

The implementation is split into `internal/store`, `internal/scan`, `internal/git`, `internal/ui`, and `internal/app`. The interactive dashboard uses Bubble Tea and Lip Gloss; the domain, store, scanner, and CLI surfaces remain independent from the UI layer.
