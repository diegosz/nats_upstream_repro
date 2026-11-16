# nats_upstream_repro

Minimal, standalone reproduction of a nats-server issue on Windows. Go only; no application code.

Pinned versions (`go.mod`): `github.com/nats-io/nats.go v1.53.1`, `github.com/nats-io/nats-server/v2 v2.14.5`.
`scripts/with-latest.sh <go test args>` reruns any test against the latest releases in a temp copy.

## filestore_lock — JetStream store files held after shutdown (nats-server, Windows)

Embedded server with `JetStream: true, StoreDir: dir`, a file-storage stream + KV with data,
`Shutdown()` + `WaitForShutdown()`, then immediately (a) start a new server on the same `StoreDir`,
(b) `os.RemoveAll(dir)`. The tests fail if the first attempt fails and log how long retries take.

`TestRestartWithLiveClient` does the same over 50 rounds (`ROUNDS=n`) while a client stays
connected through each shutdown, with a publisher and a durable pull consumer running.

```sh
go test -count=1 -v ./filestore_lock/
```

Run on `windows-latest` and `ubuntu-latest` by `.github/workflows/repro.yml` (GitHub-hosted only).
