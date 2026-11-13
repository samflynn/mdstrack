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

Tracked repositories use the authenticated `gh` CLI for the configured host to count commits whose GitHub login or commit author display name matches your settings. Either one is enough to identify you: if the username is blank the profile display name is used on its own. Counts are shown only for tracked repositories hosted on the configured GitHub instance as a deadline status followed by a 12-cell progress bar toward three commits and the actual count, such as `3 days left  ████████████  5/3`. The deadline uses the effective Deadline setting. The bar, count, and repository name are red at 0–1, yellow at 2, and green from 3 onward.

With a repository row selected, press `o` to open the application menu: `v` for VS Code, `p` for Positron, or `f` for Finder. Press `g` to open the GitHub action menu, then press `o` to open its origin URL in the default browser or `c` to copy that URL to the macOS clipboard.

Press `/` for the search chooser. Press `l` to filter the local workspace list in place; Tracking, editor, and GitHub actions continue to work on those results. Press `g` to open the GitHub repository search page. It loads every repository the authenticated `gh` account can access on the configured host; type to filter the list, use the arrow keys to navigate, and press Esc to return to the local repository list. Press `h` to hide repositories already cloned in the configured workspace, matched by their GitHub origin URL; press it again to show them. With a search result selected, press Enter, then `y` to clone directly into the workspace, or `f` to name a new folder and clone inside it. A folder clone uses `<workspace>/<new-folder>/<repository>`. Press `n` or Esc to cancel; existing destination folders are never overwritten.

Press `?` for the keyboard-shortcuts overlay; Esc closes it. The overlay is capped to the terminal height and the list keeps rendering exactly the terminal's row count at every size.

## Interface

The layout is borderless: a title line, the workspace path, one accent rule, the body, and a persistent footer. Every screen renders exactly the terminal's row and column count, so nothing is clipped or wrapped at any size, and long repository names, paths, and inputs are truncated to the available cells — measured in terminal cells rather than bytes, so CJK and emoji names stay aligned.

The Tracking section is pinned to the top of the list, so it stays visible however far you scroll. It holds still as a block — its title, its group headings, and its tracked repositories — while the remaining groups scroll beneath it. The selected row moves freely through the visible window instead of being held against the top edge, and a `n–m of total` indicator appears in the body whenever part of the list is scrolled out of view. Rows are counted across the whole list, so the first tracked row is always row 1. If the terminal is too short to hold the pinned section and a scrolled row at once, the list falls back to scrolling as a whole so the selection is never hidden.

The footer carries the key legend on the left and the refresh countdown on the right, and it is replaced while a prompt is open or a status message is showing. Legend entries are `↑/↓` move, `Enter` track, `/` search, `e` rename, `o` open, `g` GitHub, and `?` help.

Transient status messages, such as a failed action or a completed clone, clear themselves after a few seconds, so the footer always returns to the legend. A newer message replaces an older one without the older message's timer clearing it early.

`mds` respects `NO_COLOR`, so `NO_COLOR=1 mds` retains all text and state markers while suppressing ANSI colors. The commit bar is drawn with block glyphs, so its magnitude stays readable without color.

Select any Tracking or repository-group header and press `e` to rename it. Group names are saved per folder name and are applied consistently in both Tracking and the main repository list. Clear a group-name input and press Enter to restore its folder name.

While you type into a rename or settings prompt the text appears with a blinking caret, and the list above gives up the rows the prompt needs, so the prompt and what you have typed stay on screen even on a very short terminal.
