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

Press `s` in the repository list to open Settings. Use the arrow keys to select **Workspace**, **GitHub username**, **GitHub profile display name**, or **GitHub host**, then press Enter to edit the selected value.

Tracked repositories use the authenticated `gh` CLI for the configured host to count commits whose GitHub login or commit author display name matches your settings. Counts are shown only for tracked repositories hosted on the configured GitHub instance as a 12-cell progress bar toward three commits, followed by the actual count, such as `5/3`. The bar, count, and repository name are red at 0–1, yellow at 2, and green from 3 onward.
