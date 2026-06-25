package mykv

import (
	"fmt"
	"path/filepath"
	"testing"
)

var benchmarkSink *Item

func benchmarkOptions() *Options {
	return &Options{
		pageSize:       testPageSize,
		MinFillPercent: testMinPercentage,
		MaxFillPercent: testMaxPercentage,
	}
}

func benchmarkKey(i int) []byte {
	return []byte(fmt.Sprintf("%016d", i))
}

func benchmarkValue(i int) []byte {
	return []byte(fmt.Sprintf("value-%016d", i))
}

func createBenchmarkDB(b *testing.B) (*DB, func()) {
	b.Helper()

	db, err := Open(filepath.Join(b.TempDir(), "bench.db"), benchmarkOptions())
	if err != nil {
		b.Fatal(err)
	}

	return db, func() {
		if err := db.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func populateBenchmarkWriteTx(b *testing.B, count int) (*DB, *Tx, *Collection, func()) {
	b.Helper()

	db, cleanFunc := createBenchmarkDB(b)
	tx := db.WriteTx()
	collection, err := tx.CreateCollection(testCollectionName)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < count; i++ {
		if err := collection.Put(benchmarkKey(i), benchmarkValue(i)); err != nil {
			b.Fatal(err)
		}
	}
	return db, tx, collection, func() {
		tx.Rollback()
		cleanFunc()
	}
}

func BenchmarkCollectionPut(b *testing.B) {
	db, cleanFunc := createBenchmarkDB(b)
	defer cleanFunc()

	tx := db.WriteTx()
	collection, err := tx.CreateCollection(testCollectionName)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := collection.Put(benchmarkKey(i), benchmarkValue(i)); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkCollectionFindExisting(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			_, _, collection, cleanFunc := populateBenchmarkWriteTx(b, size)
			defer cleanFunc()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				item, err := collection.Find(benchmarkKey(i % size))
				if err != nil {
					b.Fatal(err)
				}
				if item == nil {
					b.Fatal("expected item")
				}
				benchmarkSink = item
			}
		})
	}
}

func BenchmarkCollectionFindMissing(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			_, _, collection, cleanFunc := populateBenchmarkWriteTx(b, size)
			defer cleanFunc()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				item, err := collection.Find(benchmarkKey(size + (i % size)))
				if err != nil {
					b.Fatal(err)
				}
				if item != nil {
					b.Fatal("unexpected item")
				}
				benchmarkSink = item
			}
		})
	}
}

func BenchmarkCollectionRemove(b *testing.B) {
	_, _, collection, cleanFunc := populateBenchmarkWriteTx(b, b.N)
	defer cleanFunc()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := collection.Remove(benchmarkKey(i)); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}
