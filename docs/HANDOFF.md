---
created_at: 2026-11-14T18:05:24Z
slug: upstream-nats-bug-repro
title: "Upstream NATS Bug Repro"
mode: resume
next_focus: "Build a standalone repro project for two NATS bugs found in the APEX v1.5 trial, prove each on CI, then file one upstream issue per bug linking it — every outward step on the operator's word"
---

# Handoff: Upstream NATS Bug Repro

> **Resume Protocol** — You are reading a handoff document. Treat it as restored session context.
>
> 1. Read the entire document before responding.
> 2. Acknowledge in one sentence ("Resuming session: Upstream NATS Bug Repro").
> 3. Present the plan below and ask the operator the open questions (Decisions needed) before
>    creating anything outside this machine.
> 4. Do NOT re-summarise the document back to the user.

## Goal

Two bugs in NATS surfaced during the APEX v1.5 trial. Neither affects `ape`'s shipped binary; both
were worked around in `ape`'s tests only. Prove each in a **separate, minimal project that has no
`ape` code**, then file one upstream issue per bug that links that project.

| ID               | Upstream repo                                      | Version seen     |
| ---------------- | -------------------------------------------------- | ---------------- |
| V15-NATS-MICRO-1 | `github.com/nats-io/nats.go` (`micro` package)     | nats.go v1.53.1  |
| V15-NATS-FS-1    | `github.com/nats-io/nats-server` (JetStream store) | v2.14.5 embedded |

Record: `$C/BACKLOG-post-trial.md` (both entries) and `$C/BRIEF.md` D919, D919+, D920
(`$C = /home/diegos/_dev/exoar/apex_process_framework_v1/v1-trial`).

## Bug 1 — V15-NATS-MICRO-1 (nats.go `micro.AddService` data race)

- **Mechanism (read from nats.go v1.53.1, research + ape, D919+ 08:30Z):** `micro.AddService`
  installs its connection-closed handler — whose `Stop` ranges `s.verbSubs` under `s.m` — **before**
  `addVerbHandlers` writes `s.verbSubs` **without** `s.m`. A connection closed while `AddService`
  runs makes the two touch the map concurrently: a `DATA RACE` under `-race`, or a fatal
  `concurrent map read and map write` without it.
- **How ape hit it:** `supervisor TestRestartBudget` closed an in-process coordinator's connection
  ≈ 0.3 s after start. Reproduced 1/20 independently by ape and research under CPU burners, 2 ×
  `DATA RACE`. Evidence: `$C/research/v15-sup-race-defa954-x20.log.txt` (contains ape paths — quote
  only the nats.go frames upstream).
- **Repro design:** an embedded `nats-server` (in-process, `-race`), a client connection, a
  goroutine calling `micro.AddService(nc, cfg)` while another closes `nc` after a short random delay;
  loop N times. Pass = the race detector fires on the nats.go frames above. Run under load
  (e.g. `stress` or `GOMAXPROCS` high + parallel `go test -count`) since it is timing-dependent.
- **Before filing:** check the latest nats.go release and `main` — if the order was fixed, report
  nothing (note it in BACKLOG instead). Search existing issues for `AddService race verbSubs`.

## Bug 2 — V15-NATS-FS-1 (Windows: store files stay locked after shutdown)

- **Observation (D919+ 14:29Z/15:50Z, D920):** on GitHub's `windows-latest` runners, an embedded
  nats-server with JetStream, after `Shutdown()` + `WaitForShutdown()`, still holds its store files.
  A new server on the **same store directory** fails at JetStream's store rename with
  `Access is denied`; `t.TempDir()` cleanup reports the directory left behind. Not seen on Linux.
- **ape's workaround (test-only, `internal/natstest`, ape d584e44):** `Restart` retries the
  store-in-use start error for up to 20 s and logs `started again after …`. Five consecutive green
  Windows runs on it (bar runs 43257100734, 43260921130, 43265401941, 43269829083, 43274229580) —
  none exhausted 20 s. The lock's real lifetime is in those jobs' logs only (403 to ape's session);
  the operator may read the `started again after` lines to quantify it.
- **Repro design:** embedded server with `JetStream: true, StoreDir: <dir>`, create a stream and
  write messages (file storage), `Shutdown()` + `WaitForShutdown()`, then immediately start a second
  server on the same `StoreDir` (and separately try `os.RemoveAll(dir)`). Record the error and how
  long until a retry succeeds. Run on GitHub-hosted `windows-latest` (and `ubuntu-latest` as the
  control). Pass = Windows fails/delays, Linux does not.
- **Before filing:** test the latest nats-server release too; search issues for
  `windows WaitForShutdown store locked` / `Access is denied filestore`.

## Decisions needed (ask the operator first)

1. **Where the repro repo lives:** owner (`diegosz` personal vs an org), name, public (upstream
   maintainers must be able to read it) — creating it is outward-facing.
2. **CI runners:** use **GitHub-hosted** `windows-latest` / `ubuntu-latest` only. Never bare
   `runs-on: [self-hosted]` in exoar repos (the exoar-dev group serves every exoar repo; D871 leak).
   No secrets are needed.
3. **Who files:** the issues go out under the operator's GitHub account; the operator approves each
   issue's text before it is posted.
4. **Whether to also offer a fix PR** for Bug 1 (reorder: write `verbSubs` under `s.m`, or install
   the closed handler after `addVerbHandlers`). Optional.

## Steps

1. Local: scaffold the repro (Go module, `go.mod` pinning nats.go v1.53.1 and nats-server/v2
   v2.14.5 — the versions `ape` 1.5.0 uses; then a second run on latest). Prove Bug 1 on Linux
   locally with `-race`. $0.
2. On the word: create the repo, push, run the Windows/Linux workflow for Bug 2; capture logs.
3. Draft both issues (title, versions, minimal code, observed output, link to the repo + run),
   plain and short; the operator reviews.
4. On the word: file them; record the issue URLs in `$C/BACKLOG-post-trial.md` beside each entry and
   a BRIEF line.

## Out of Scope

- Nothing about `ape`'s own code; the workarounds stay as shipped in 1.5.0.
- No push, repo creation, or issue filing without the operator's word.
- Never read `~/hot` or the build repo's `hot/`; never create a `CLAUDE.md` in the build repo.
