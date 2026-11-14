# nats_upstream_repro

Standalone reproductions of two NATS bugs found in the APEX v1.5 trial, for upstream issues. The plan,
the bugs and the decisions to ask are in `docs/HANDOFF.md`.

- No `ape` code here: minimal Go only, pinned to the versions under test.
- Local work ($0) is fine. Creating a remote repo, pushing, running CI, or filing an issue needs the
  operator's explicit word, asked in this session.
- CI: GitHub-hosted runners only (`windows-latest`, `ubuntu-latest`); never `runs-on: [self-hosted]`.
- Commits: no Claude attribution or "Generated with Claude Code" lines.
- Report progress in `docs/STATUS.md` (newest heading on top).
