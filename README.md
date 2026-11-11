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

On first launch, enter the path to the folder containing your UBC MDS repositories and your GitHub username. Both values are stored in your user configuration directory and every directory below the workspace containing a `.git` folder is listed.

Press `s` in the repository list to open Settings. Use the arrow keys to select **Workspace** or **GitHub username**, then press Enter to edit the selected value.
