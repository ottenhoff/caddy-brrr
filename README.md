# caddy-brrr

Third-party Brotli encoder module for Caddy backed by `github.com/molecule-man/go-brrr`.

This repository is intentionally a proving ground before proposing any Caddy core dependency change. Its tests and benchmarks use Caddy's encode harness shape to demonstrate:

- encoder contract compliance,
- `encode` middleware behaviour,
- throughput, allocations, and compression ratio against Caddy gzip/zstd.

Run:

```sh
go test ./...
go test -bench=. -benchmem ./...
```

Dependencies are resolved from the module proxy (`github.com/molecule-man/go-brrr` v1.0.0). This repo does not use `replace` directives for local checkouts.
