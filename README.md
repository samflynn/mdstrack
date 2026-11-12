# mds

`mds` is a full-screen terminal application that lists Git repositories inside a UBC MDS folder.

## Install

```bash
go install ./cmd/mds
```

Ensure Go's binary directory is on your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Then start it with:

```bash
mds
```

On first launch, enter the path to the folder containing your UBC MDS repositories, GitHub username, GitHub profile display name, and GitHub host. The host defaults to `github.ubc.ca`. These values are stored in your user configuration directory and every directory below the workspace containing a `.git` folder is listed.

Press `s` in the repository list to open Settings. Use the arrow keys to select **Workspace**, **GitHub username**, **GitHub profile display name**, **GitHub host**, **Deadline**, or **Refresh interval**, then press Enter to edit the selected value. Deadline defaults to the upcoming Saturday. Enter a one-off `YYYY-MM-DD` override; once that date has passed, the effective deadline automatically returns to the upcoming Saturday. Clear the override input and press Enter to return to the default immediately. Refresh interval defaults to 5 minutes; it rescans the workspace, tracked commit progress, and cloned-repository state. The home footer shows the last refresh and live countdown to the next one.

Tracked repositories use the authenticated `gh` CLI for the configured host to count commits whose GitHub login or commit author display name matches your settings. Counts are shown only for tracked repositories hosted on the configured GitHub instance as a deadline status followed by a 12-cell progress bar toward three commits and the actual count, such as `3 days left  ████████████  5/3`. The deadline uses the effective Deadline setting. The bar, count, and repository name are red at 0–1, yellow at 2, and green from 3 onward.

With a repository row selected, press `o` to open the application menu: `v` for VS Code, `p` for Positron, or `f` for Finder. Press `g` to open the GitHub action menu, then press `o` to open its origin URL in the default browser or `c` to copy that URL to the macOS clipboard.

Press `/` for the search chooser. Press `l` to filter the local workspace list in place; Tracking, editor, and GitHub actions continue to work on those results. Press `g` to open the GitHub repository search page. It loads every repository the authenticated `gh` account can access on the configured host; type to filter the list, use the arrow keys to navigate, and press Esc to return to the local repository list. Press `h` to hide repositories already cloned in the configured workspace, matched by their GitHub origin URL; press it again to show them. With a search result selected, press Enter, then confirm with `y`, to clone it into the workspace folder. Press `n` or Esc to cancel; existing destination folders are never overwritten.

Press `?` for the keyboard-shortcuts overlay. `mds` respects `NO_COLOR`, so `NO_COLOR=1 mds` retains all text and state markers while suppressing ANSI colors.

The home view is a terminal-native workspace console: a live status header shows the deadline and refresh timing, Tracking is presented before the course groups, and a persistent command bar keeps core shortcuts visible.
