# DRAFT — NOT READY, DO NOT FILE

> Status 2026-11-16: the repro in this repo does **not** reproduce the bug. Both CI runs were green
> on `windows-latest` and `ubuntu-latest`, for v2.14.5 and v2.15.3:
> https://github.com/diegosz/nats_upstream_repro/actions/runs/43766782866 (5 idle restarts) and
> https://github.com/diegosz/nats_upstream_repro/actions/runs/43893399879 (50 restarts with a live
> client, publisher and durable consumer). An issue needs a red Windows run to
> link, so the text below is a skeleton to finish once one exists. Fill every `<…>`.

---

**Title:** Windows: JetStream store files still held after `Shutdown()` + `WaitForShutdown()` (embedded server)

**Versions:** nats-server `<v2.14.5 / v2.15.3>` (embedded), nats.go `<…>`, Go `<…>`,
GitHub-hosted `windows-latest` (`<image version from the run log>`). Not seen on `ubuntu-latest`.

**What happens**

An embedded server with `JetStream: true` and a `StoreDir` (file storage) is stopped with
`Shutdown()` followed by `WaitForShutdown()`. Straight afterwards, a new server on the same
`StoreDir` fails to enable JetStream:

```
<exact error text from the red run, e.g. "... Access is denied.">
```

and `os.RemoveAll(StoreDir)` fails as well, until about `<N> s` later. On Linux both succeed on the
first attempt.

**Expected:** once `WaitForShutdown()` returns, every file handle in the store directory is
closed.

**Reproduction:** https://github.com/diegosz/nats_upstream_repro (`filestore_lock/lock_test.go`,
`go test -count=1 -v ./filestore_lock/`), red run: `<link>`.

```go
<the minimal steps, about 15 lines, copied from the test once it reproduces>
```

**Notes:** `<which handles were still open, if known; the remaining files the test lists>`
