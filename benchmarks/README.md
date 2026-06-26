# Benchmark Reports

Run benchmarks with:

```sh
make bench
```

`make bench` runs Go benchmarks, stores the raw output and parsed medians under `benchmarks/runs/<system-id>/`, and regenerates `benchmarks/report.md`.

The system id is built from `goos`, `goarch`, and `cpu` values emitted by Go's benchmark runner. Comparisons are only made between runs with identical system metadata. Different hardware appears as a separate system in the report and is not compared.

Useful options:

```sh
make bench BENCH_COUNT=5
make bench BENCH=CollectionFind
```
