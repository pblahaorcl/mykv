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

For the results below, the benchmark was run with three samples:

```sh
go test -run '^$' -bench=. -benchmem -count=3 ./...
```

Environment:

- OS: darwin
- Architecture: arm64
- CPU: Apple M4 Pro
- Go package: `mykv`

The benchmark setup uses deterministic fixed-width keys and values. Lookup benchmarks compare trees preloaded with 100, 1000, and 10000 items. These benchmarks measure in-transaction tree operations and do not include crash recovery or reopen-after-commit behavior.

## Median Results

| Benchmark | Size | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| Put | growing | 502.2 | 349 | 12 |
| Find existing | 100 | 114.8 | 39 | 2 |
| Find existing | 1000 | 158.2 | 76 | 4 |
| Find existing | 10000 | 159.4 | 78 | 4 |
| Find missing | 100 | 98.58 | 40 | 3 |
| Find missing | 1000 | 129.9 | 80 | 5 |
| Find missing | 10000 | 134.7 | 80 | 5 |
| Remove | growing | 390.2 | 210 | 7 |

## Comparison

| Operation | Median ns/op | Relative to Put |
| --- | ---: | ---: |
| Find missing, 100 items | 98.58 | 0.20x |
| Find existing, 100 items | 114.8 | 0.23x |
| Find missing, 10000 items | 134.7 | 0.27x |
| Find existing, 10000 items | 159.4 | 0.32x |
| Remove | 390.2 | 0.78x |
| Put | 502.2 | 1.00x |

Lookup cost grows modestly across the tested sizes. Existing-key lookup increased from 114.8 ns/op at 100 items to 159.4 ns/op at 10000 items, about 39% slower. Missing-key lookup increased from 98.58 ns/op to 134.7 ns/op, about 37% slower.

## Raw Output

```text
goos: darwin
goarch: arm64
pkg: mykv
cpu: Apple M4 Pro
BenchmarkCollectionPut-12                         2534205   502.2 ns/op   349 B/op   12 allocs/op
BenchmarkCollectionPut-12                         2491239   498.4 ns/op   349 B/op   12 allocs/op
BenchmarkCollectionPut-12                         2507870   504.0 ns/op   349 B/op   12 allocs/op
BenchmarkCollectionFindExisting/items=100-12     10553895   114.8 ns/op    39 B/op    2 allocs/op
BenchmarkCollectionFindExisting/items=100-12     10308031   114.6 ns/op    39 B/op    2 allocs/op
BenchmarkCollectionFindExisting/items=100-12     10597983   116.5 ns/op    39 B/op    2 allocs/op
BenchmarkCollectionFindExisting/items=1000-12     7534921   160.4 ns/op    76 B/op    4 allocs/op
BenchmarkCollectionFindExisting/items=1000-12     7227230   158.2 ns/op    76 B/op    4 allocs/op
BenchmarkCollectionFindExisting/items=1000-12     7463888   151.6 ns/op    76 B/op    4 allocs/op
BenchmarkCollectionFindExisting/items=10000-12    7436548   159.3 ns/op    78 B/op    4 allocs/op
BenchmarkCollectionFindExisting/items=10000-12    7217797   159.4 ns/op    78 B/op    4 allocs/op
BenchmarkCollectionFindExisting/items=10000-12    7503126   168.9 ns/op    78 B/op    4 allocs/op
BenchmarkCollectionFindMissing/items=100-12      12175838    98.58 ns/op   40 B/op    3 allocs/op
BenchmarkCollectionFindMissing/items=100-12      12125672    98.20 ns/op   40 B/op    3 allocs/op
BenchmarkCollectionFindMissing/items=100-12      12400088    99.76 ns/op   40 B/op    3 allocs/op
BenchmarkCollectionFindMissing/items=1000-12      9156867   129.9 ns/op    80 B/op    5 allocs/op
BenchmarkCollectionFindMissing/items=1000-12      8892705   129.1 ns/op    80 B/op    5 allocs/op
BenchmarkCollectionFindMissing/items=1000-12      9476539   130.5 ns/op    80 B/op    5 allocs/op
BenchmarkCollectionFindMissing/items=10000-12     9011054   134.5 ns/op    80 B/op    5 allocs/op
BenchmarkCollectionFindMissing/items=10000-12     8962725   135.2 ns/op    80 B/op    5 allocs/op
BenchmarkCollectionFindMissing/items=10000-12     8906900   134.7 ns/op    80 B/op    5 allocs/op
BenchmarkCollectionRemove-12                      3270106   390.2 ns/op   210 B/op    7 allocs/op
BenchmarkCollectionRemove-12                      3296300   390.6 ns/op   210 B/op    7 allocs/op
BenchmarkCollectionRemove-12                      3331771   386.8 ns/op   210 B/op    7 allocs/op
PASS
ok      mykv    44.120s
?       mykv/cmd/mykv  [no test files]
```
