# Storage layout, project identity, and forks

Limbo keeps task state in a per-project **central store** outside the working
tree. This page explains where that store lives, how the project identity that
selects it is derived, and the consequences for clones, worktrees, and forks.

## Where the store lives

Each project gets its own directory under a platform-conventional data root:

- **macOS:** `~/Library/Application Support/limbo/projects/<id>/`
- **Linux:** `${XDG_DATA_HOME}/limbo/projects/<id>/`, defaulting to
  `~/.local/share/limbo/projects/<id>/` when `XDG_DATA_HOME` is unset or empty.

Set the `LIMBO_HOME` environment variable to override the platform default; the
store then lives at `${LIMBO_HOME}/projects/<id>/`. `LIMBO_HOME` is resolved once
per invocation and redirects both the resolved storage root and the destination
that `limbo migrate` writes to.

Inside a project's storage root you will find `tasks.json` (the task index, with
a top-level monotonically increasing `revision` counter), a `context/` directory
of per-task markdown sidecars, and lock/backup bookkeeping files used by the
JSON backend.

## How the project ID is derived

The `<id>` segment above is resolved with a fixed priority hierarchy:

1. **`.limbo-id` override.** If a file named `.limbo-id` exists at the resolved
   project root, its first non-empty trimmed line is the project ID. The
   contents are opaque — limbo does not validate the format beyond requiring it
   be non-empty. This takes precedence over everything below.
2. **Git first-commit hash.** Otherwise, if the project root is inside a git
   working tree with at least one commit, the ID is the 40-character lowercase
   hex SHA of the first commit reachable from `HEAD` — equivalent to
   `git rev-list --max-parents=0 HEAD | tail -1`.
3. **Generated UUID.** Otherwise (a non-git directory, or a git repo with no
   commits), `limbo init` generates a UUID v4, writes it to `.limbo-id` at the
   project root, and uses it as the ID. Because the generated ID is persisted to
   `.limbo-id`, it is stable across subsequent runs.

## Forks share a project ID — by design

Because the default ID is the **first-commit hash**, it is stable across clones,
remote-URL changes, and git worktrees: every worktree of the same repository
resolves to the same central store, so switching between worktrees preserves task
continuity.

A direct consequence is that **forks of the same upstream share a project ID**,
and therefore share the same central store. The first commit of a fork is the
first commit of the upstream it was forked from, so the derived hash is
identical. This is intended: a fork is usually a continuation of the same line of
work, and sharing the task store keeps that continuity.

### Giving a fork its own store

If you want a fork (or any clone) to have a **separate** task store, create a
`.limbo-id` override at the project root with a distinct value:

```sh
echo my-fork-specific-id > .limbo-id
```

Because the `.limbo-id` override sits at the top of the priority hierarchy, the
fork now resolves to `projects/my-fork-specific-id/` and gets an isolated store,
independent of the upstream's first-commit-derived ID.
