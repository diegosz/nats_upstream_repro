# Status

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
