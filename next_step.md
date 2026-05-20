# Next Step: Selected Move Folder Picker

## Agent Workflow Contract

This file is the handoff contract for the implementation agent.

- Do not make commits.
- Do not push.
- Do not rewrite history.
- Leave all implementation changes as a normal workspace diff.
- After finishing, report:
  - what files changed;
  - what behavior changed;
  - what tests/checks were run and their result;
  - any blocker or skipped acceptance criterion.
- The reviewer/maintainer will inspect the diff, update this file for the next task, and make the commit.
- If the task is ambiguous, stop and write the specific question instead of implementing a broad guess.

## Goal

Add an explicit folder picker for moving selected files and folders in the embedded web UI.

The current UI supports drag-and-drop moves, including selected items, but bulk move still depends on drag-and-drop. The next step is to add a reliable button-driven move flow for selected items.

## Current State

- `PATCH /files/bulk-move` exists and accepts:
  - `ids`;
  - `parent_id`, where an empty string means root.
- `POST /files/bulk-delete` exists for selected-item delete.
- `PATCH /files/{id}` supports rename and single-item move.
- The embedded UI has selected-item checkboxes, selected delete, compact row actions, details-based rename, and drag-to-folder/ancestor/root move.
- `docs/development/files.md` documents the current file management API.

## Required Behavior

1. Add a `Move selected` control to the selection bar.
2. Open a modal or compact picker that lets the user choose:
   - root;
   - the current folder's ancestors;
   - visible folders in the current listing.
3. Use `PATCH /files/bulk-move` for the selected IDs.
4. Prevent invalid self moves in the UI when the selected set contains the target folder; keep backend validation as the final guard.
5. Refresh the listing and clear selection after a successful move.
6. Keep shared view read-only: the picker must only be available in the owner view.
7. Do not change upload, sharing, recovery, or worker behavior in this task.

## Suggested Implementation

Keep the first implementation small and local to the embedded UI:

- extend the selection bar with a `Move selected` button;
- add a modal that reuses `state.folderStack` and the currently rendered file list;
- include root as a target;
- list visible child folders as targets;
- call the existing `moveFiles(ids, parentID)` helper;
- disable targets that are part of the selected set.

If a full folder tree picker is too broad for this pass, document it as a later enhancement and keep this task scoped to current ancestors plus visible folders.

## Files To Inspect

- `backend/internal/httpserver/static/index.html`
- `backend/internal/files/http.go`
- `backend/internal/files/store.go`
- `docs/development/files.md`
- `TeleVault-plan.md`

## Tests To Add Or Update

Add focused coverage where practical:

- embedded JavaScript syntax check;
- Go tests if backend behavior changes;
- update docs if the picker changes documented behavior.

Backend behavior should not need new tests if the task only wires the existing bulk move endpoint into the UI.

## Verification

Run:

```sh
cd backend
go test ./... -count=1
```

Then validate the embedded JavaScript syntax, for example:

```sh
awk '/<script>/{flag=1;next} /<\/script>/{flag=0} flag' backend/internal/httpserver/static/index.html > /tmp/televault-index.js
node --check /tmp/televault-index.js
```

Finally run:

```sh
git diff --check
```

## Acceptance Criteria

- Selected owner-view items can be moved through a button-driven picker.
- Root, ancestors, and visible child folders are available as targets.
- Invalid selected-target moves are disabled or blocked in the UI.
- Successful move refreshes the file list and clears selection.
- Shared view does not expose owner-only move controls.
- `go test ./... -count=1` passes.
- Embedded JavaScript syntax check passes.
- `git diff --check` passes.
