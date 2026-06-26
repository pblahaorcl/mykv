# Benchmarks

Benchmarks were added in `benchmark_test.go` for the main collection operations:

- sequential inserts with `Collection.Put`
- existing-key lookups with `Collection.Find`
- missing-key lookups with `Collection.Find`
- sequential deletes with `Collection.Remove`

Run them with:

```sh
make bench
```

`make bench` stores raw benchmark output, parsed JSON, and a generated Markdown report under [`benchmarks/`](../benchmarks/README.md). The report compares the current run to the previous run from the same hardware only. Runs from different `goos`, `goarch`, or `cpu` values are listed separately and are not compared against each other.

By default, the benchmark runner collects three samples with:

```sh
go test -run '^$' -bench=. -benchmem -count=3 ./...
```

The benchmark setup uses deterministic fixed-width keys and values. Lookup benchmarks compare trees preloaded with 100, 1000, and 10000 items. These benchmarks measure in-transaction tree operations and do not include crash recovery or reopen-after-commit behavior.
