# Status

## 2026-11-15 — Pushed; CI green on Windows too: Bug 2 NOT reproduced

- Pushed `main` (52fde0f, no tags) to the public `diegosz/nats_upstream_repro`. The push triggered
  `repro.yml`, run 43766782866
  (https://github.com/diegosz/nats_upstream_repro/actions/runs/43766782866): **all 4 jobs
  succeeded**.

  | Job                                   | Result  | Duration |
  | ------------------------------------- | ------- | -------- |
  | windows-latest, pinned (v2.14.5)      | success | 77 s     |
  | windows-latest, latest (v2.15.3)      | success | 69 s     |
  | ubuntu-latest, pinned (v2.14.5)       | success | 44 s     |
  | ubuntu-latest, latest (v2.15.3)       | success | 44 s     |

- So on Windows too: 5 restarts in a row on the same `StoreDir` and an immediate `RemoveAll` all
  succeeded first time. The tests `t.Errorf` on any first-attempt failure, so green means no
  lock was seen. GitHub returns 403 for the job logs to unauthenticated reads, so I have no
  per-round timings; the operator can read them on the run page.
- Likely gap against ape's case: `populate` closes its client before `Shutdown()`, so the server
  stops with no connections, consumers or in-flight writes. ape shut down with live clients
  attached. Other candidates: more streams/consumers, publishing during shutdown, many more rounds.
- `docs/ISSUE-nats-server.md` is a skeleton headed "DRAFT — NOT READY, DO NOT FILE". There is no
  red run to link yet, so there is no bug report to file.
- Proposed next step (needs the operator's word: a new push and CI run): extend the repro to shut
  down with a live client, a durable consumer and a publisher still running, loop 50 rounds, and
  rerun on `windows-latest`. If that's still green, fall back to the operator reading the
  `started again after …` lines in ape's five Windows runs.

## 2026-11-15 — Decisions answered; Bug 1 dropped; waiting on the empty repo

Operator's answers (relayed by the conductor): (1) public `diegosz/nats_upstream_repro`; operator
creates it empty on the web, then I push. (2) GitHub-hosted `windows-latest` + `ubuntu-latest` only.
(3) Bug 1 is not reported and is out of this repo's scope (the conductor moves `ape` to nats.go
v1.55.0). (4) Bug 2: I draft `docs/ISSUE-nats-server.md` after the CI result; the operator files it.
Nothing else outward.

- Removed `micro_race/`, the Bug 1 evidence and the `micro-race` CI job (all still in commit
  e7ddb97). Findings kept in the entry below.
- `repro.yml` now only runs `filestore-lock`: {windows-latest, ubuntu-latest} × {pinned v2.14.5,
  latest}, on push to `main` and on `workflow_dispatch`.
- SSH to github.com authenticates as `diegosz`. Next: push once the empty repo exists, then read
  the run.
- 2026-11-15T14:50Z: the 2 h push watcher timed out; `diegosz/nats_upstream_repro` still does not
  exist. Nothing pushed. Waiting for the operator to create the empty public repo.

## 2026-11-14 — Local phase done ($0); waiting on the four Decisions

Nothing has left this machine: no remote, no push, no CI, no issue. Upstream checks were anonymous
read-only GitHub API / raw / Go proxy reads.

**Scaffold.** Module `github.com/diegosz/nats_upstream_repro`, go 1.27.3; pinned `nats.go v1.53.1`,
`nats-server/v2 v2.14.5`. Packages `micro_race/` (Bug 1), `filestore_lock/` (Bug 2);
`scripts/with-latest.sh` (latest releases in a temp copy); `.github/workflows/repro.yml`
(GitHub-hosted `windows-latest` / `ubuntu-latest` only; not pushed). `GOOS=windows go vet` clean.

**Bug 1 (V15-NATS-MICRO-1) — proved locally on v1.53.1, and already fixed upstream.**
- Mechanism confirmed in v1.53.1 source: `wrapConnectionEventCallbacks` (service.go:350, closed
  handler → `Stop`, ranges `verbSubs` under `s.m` at :726) runs before `addVerbHandlers` →
  `addInternalHandler` writes `s.verbSubs[name]` with no lock (:681).
- `go test -race` (2000 iterations, close after 0–200 µs), idle machine: DATA RACE in 8/10 runs,
  exactly the frames above. With 8 CPU burners + 4 parallel binaries: 11/12 runs.
- Without `-race`, under load (12 parallel, ITER=5000): `fatal error: concurrent map iteration and
  map write` in 8/12; the other 4 hit a nil-pointer panic in `Stop` (:733). 0/12 passed.
- The nil panic is a second defect on the same path (a failed `Subscribe` stores `nil` in
  `verbSubs`): nats.go #2154, merged 2026-09-24.
- Upstream: the map write moved under `s.m` in nats.go **#2174** (merged 2026-10-13). v1.54.0 still
  has the bug; **v1.55.0** (latest, 2026-10-23) and `main` (91d0049, 2026-11-12) have the fix.
  Control: v1.55.0 + nats-server v2.15.3 → 10/10 clean `-race` runs.
- Issue search (`verbSubs`, `micro race AddService`, `micro data race closed handler`): no open
  issue; only the fix PRs above.
- **Per HANDOFF, nothing to file for Bug 1.** Note it in `$C/BACKLOG-post-trial.md` instead:
  fixed in nats.go v1.55.0 (#2174, #2154); `ape` can drop its test workaround on upgrade. Writing
  that note is outside this repo, so it waits for the operator's word.
- Evidence: `docs/evidence/bug1-v1.53.1-race-load.log`, `bug1-v1.53.1-norace-load.log`,
  `bug1-v1.55.0-race-control.log`.

**Bug 2 (V15-NATS-FS-1) — repro written, Linux control green, Windows needs CI.**
- `TestRestartSameStoreDir` (5 rounds of shutdown → immediate restart on the same `StoreDir`, retry
  up to 60 s, logs the delay) and `TestRemoveStoreDir` (shutdown → immediate `os.RemoveAll`, lists
  the files left). Linux: PASS on v2.14.5 and on v2.15.3 (latest), first attempt every time.
- Latest nats-server: v2.15.3 (2026-11-11); also v2.14.10 on the 2.14 line. No release note since
  v2.14.5 mentions Windows store handles after shutdown.
- Issue search (`windows Access is denied`, `windows WaitForShutdown`, `windows filestore lock
  shutdown`, `windows store directory in use`): no match for this bug.

**Next (needs the operator's word):** the four Decisions in HANDOFF; then create the repo, push and
run `repro.yml` for the Windows result.
