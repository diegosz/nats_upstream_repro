# nats_upstream_repro

Minimal, standalone reproductions of two NATS issues. Go only; no application code.

Pinned versions (`go.mod`): `github.com/nats-io/nats.go v1.53.1`, `github.com/nats-io/nats-server/v2 v2.14.5`.
`scripts/with-latest.sh <go test args>` reruns any test against the latest releases in a temp copy.

## micro_race — `micro.AddService` races with a connection close (nats.go)

`AddService` installs its closed handler (which calls `Stop`, ranging `s.verbSubs` under `s.m`)
before `addInternalHandler` writes `s.verbSubs` without `s.m`. Closing the connection while
`AddService` runs touches the map concurrently.

```sh
go test -race -count=1 -v ./micro_race/          # v1.53.1: WARNING: DATA RACE (service.go:681 vs :726)
go test -count=1 ./micro_race/                   # v1.53.1, under load: fatal error: concurrent map iteration and map write
scripts/with-latest.sh -race -v ./micro_race/    # v1.55.0: PASS
```

Fixed upstream in nats.go v1.55.0 (nats-io/nats.go#2174 for the map write; #2154 for a nil
`*Subscription` panic hit by the same test).

## filestore_lock — JetStream store files held after shutdown (nats-server, Windows)

Embedded server with `JetStream: true, StoreDir: dir`, a file-storage stream + KV with data,
`Shutdown()` + `WaitForShutdown()`, then immediately (a) start a new server on the same `StoreDir`,
(b) `os.RemoveAll(dir)`. The tests fail if the first attempt fails and log how long retries take.

```sh
go test -count=1 -v ./filestore_lock/
```

Run on `windows-latest` and `ubuntu-latest` by `.github/workflows/repro.yml` (GitHub-hosted only).
