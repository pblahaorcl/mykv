package main

import "testing"

func TestParseBenchmarkSamplesAndSummarize(t *testing.T) {
	output := `goos: darwin
goarch: arm64
pkg: mykv
cpu: Apple M4 Pro
BenchmarkCollectionPut-12       10  100.0 ns/op  20 B/op  2 allocs/op
BenchmarkCollectionPut-12       10  120.0 ns/op  24 B/op  4 allocs/op
BenchmarkCollectionPut-12       10  110.0 ns/op  22 B/op  3 allocs/op
`

	samples := parseBenchmarkSamples(output)
	if len(samples) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(samples))
	}

	results := summarize(samples)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	result := results[0]
	if result.Name != "BenchmarkCollectionPut" {
		t.Fatalf("unexpected benchmark name %q", result.Name)
	}
	if result.MedianNSPerOp != 110 {
		t.Fatalf("unexpected ns/op median %v", result.MedianNSPerOp)
	}
	if result.MedianBytesPerOp == nil || *result.MedianBytesPerOp != 22 {
		t.Fatalf("unexpected B/op median %v", result.MedianBytesPerOp)
	}
	if result.MedianAllocsPerOp == nil || *result.MedianAllocsPerOp != 3 {
		t.Fatalf("unexpected allocs/op median %v", result.MedianAllocsPerOp)
	}
}

func TestPreviousSameHardwareIgnoresDifferentCPU(t *testing.T) {
	current := storedRun{
		Timestamp: "2026-06-26T12:00:00Z",
		DataFile:  "runs/darwin-arm64-apple-m4-pro/current.json",
		System: systemInfo{
			GOOS:   "darwin",
			GOARCH: "arm64",
			CPU:    "Apple M4 Pro",
		},
	}
	matching := storedRun{
		Timestamp: "2026-06-26T11:00:00Z",
		DataFile:  "runs/darwin-arm64-apple-m4-pro/previous.json",
		System:    current.System,
	}
	differentCPU := storedRun{
		Timestamp: "2026-06-26T11:30:00Z",
		DataFile:  "runs/darwin-arm64-apple-m3/previous.json",
		System: systemInfo{
			GOOS:   "darwin",
			GOARCH: "arm64",
			CPU:    "Apple M3",
		},
	}

	previous := previousSameHardware(current, []storedRun{matching, differentCPU, current})
	if previous == nil {
		t.Fatal("expected previous matching run")
	}
	if previous.DataFile != matching.DataFile {
		t.Fatalf("expected matching hardware run, got %s", previous.DataFile)
	}
}
