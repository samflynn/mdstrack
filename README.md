# mdstrack

`mds` is a narrow terminal dashboard for the repositories you committed to work on this week. It discovers local git repositories, tracks a weekly assignment, counts your non-merge commits against a fixed target of three, and stores state atomically as JSON.

## Install and run

```bash
go install ./cmd/mds
mds setup MDS ~/Documents/MDS you@student.ubc.ca "Your Name"
mds
```

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

The implementation is split into `internal/store`, `internal/scan`, `internal/git`, and `internal/app`. It currently uses a standard-library interactive terminal loop because this build environment cannot resolve Bubble Tea's `golang.org/x/*` dependencies. The domain, store, scanner, and CLI surfaces are independent of the UI layer so Bubble Tea can replace the presentation loop without changing data or git behavior.
