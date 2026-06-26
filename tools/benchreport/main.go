package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const dataVersion = 1

type config struct {
	benchPattern string
	runPattern   string
	count        int
	outDir       string
}

type storedRun struct {
	Version   int               `json:"version"`
	Timestamp string            `json:"timestamp"`
	Command   []string          `json:"command"`
	GoVersion string            `json:"go_version"`
	GitCommit string            `json:"git_commit,omitempty"`
	GitDirty  *bool             `json:"git_dirty,omitempty"`
	System    systemInfo        `json:"system"`
	RawOutput string            `json:"raw_output"`
	Results   []benchmarkResult `json:"results"`

	DataFile string `json:"-"`
}

type systemInfo struct {
	ID     string `json:"id"`
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
	CPU    string `json:"cpu"`
}

type benchmarkResult struct {
	Name              string            `json:"name"`
	Samples           []benchmarkSample `json:"samples"`
	MedianNSPerOp     float64           `json:"median_ns_per_op"`
	MedianBytesPerOp  *float64          `json:"median_bytes_per_op,omitempty"`
	MedianAllocsPerOp *float64          `json:"median_allocs_per_op,omitempty"`
}

type benchmarkSample struct {
	Iterations  int      `json:"iterations"`
	NSPerOp     float64  `json:"ns_per_op"`
	BytesPerOp  *float64 `json:"bytes_per_op,omitempty"`
	AllocsPerOp *float64 `json:"allocs_per_op,omitempty"`
}

type parsedSample struct {
	name   string
	sample benchmarkSample
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg := parseFlags()
	if cfg.count < 1 {
		return fmt.Errorf("count must be at least 1")
	}

	args := []string{
		"test",
		"-run=" + cfg.runPattern,
		"-bench=" + cfg.benchPattern,
		"-benchmem",
		"-count=" + strconv.Itoa(cfg.count),
		"./...",
	}

	output, err := exec.Command("go", args...).CombinedOutput()
	fmt.Print(string(output))
	if err != nil {
		return fmt.Errorf("benchmark command failed: %w", err)
	}

	system := parseSystemInfo(string(output))
	samples := parseBenchmarkSamples(string(output))
	if len(samples) == 0 {
		return fmt.Errorf("no benchmark samples found in output")
	}

	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339)
	stamp := now.Format("20060102T150405Z")
	run := storedRun{
		Version:   dataVersion,
		Timestamp: timestamp,
		Command:   append([]string{"go"}, args...),
		GoVersion: commandOutput("go", "version"),
		GitCommit: commandOutput("git", "rev-parse", "HEAD"),
		GitDirty:  gitDirty(),
		System:    system,
		Results:   summarize(samples),
	}

	if err := writeRun(cfg.outDir, stamp, string(output), &run); err != nil {
		return err
	}

	runs, err := loadRuns(cfg.outDir)
	if err != nil {
		return err
	}
	current := findRun(runs, run.DataFile)
	if current == nil {
		current = &run
	}
	if err := writeReport(cfg.outDir, *current, runs); err != nil {
		return err
	}

	fmt.Printf("Stored benchmark run: %s\n", filepath.ToSlash(filepath.Join(cfg.outDir, run.DataFile)))
	fmt.Printf("Updated benchmark report: %s\n", filepath.ToSlash(filepath.Join(cfg.outDir, "report.md")))
	return nil
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.benchPattern, "bench", ".", "benchmark regexp passed to go test -bench")
	flag.StringVar(&cfg.runPattern, "run", "^$", "test regexp passed to go test -run")
	flag.IntVar(&cfg.count, "count", 3, "number of benchmark samples to collect")
	flag.StringVar(&cfg.outDir, "out", "benchmarks", "benchmark report directory")
	flag.Parse()
	return cfg
}

func commandOutput(name string, args ...string) string {
	output, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func gitDirty() *bool {
	output, err := exec.Command("git", "status", "--short").Output()
	if err != nil {
		return nil
	}
	dirty := strings.TrimSpace(string(output)) != ""
	return &dirty
}

func parseSystemInfo(output string) systemInfo {
	info := systemInfo{
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,
		CPU:    "unknown",
	}

	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "goos":
			info.GOOS = value
		case "goarch":
			info.GOARCH = value
		case "cpu":
			if value != "" {
				info.CPU = value
			}
		}
	}

	info.ID = strings.Join([]string{slug(info.GOOS), slug(info.GOARCH), slug(info.CPU)}, "-")
	return info
}

func parseBenchmarkSamples(output string) []parsedSample {
	var samples []parsedSample
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.HasPrefix(fields[0], "Benchmark") {
			continue
		}

		iterations, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		sample := benchmarkSample{Iterations: iterations}
		foundNS := false
		for i := 2; i+1 < len(fields); i += 2 {
			value, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				continue
			}
			switch fields[i+1] {
			case "ns/op":
				sample.NSPerOp = value
				foundNS = true
			case "B/op":
				v := value
				sample.BytesPerOp = &v
			case "allocs/op":
				v := value
				sample.AllocsPerOp = &v
			}
		}
		if !foundNS {
			continue
		}

		samples = append(samples, parsedSample{
			name:   normalizeBenchmarkName(fields[0]),
			sample: sample,
		})
	}
	return samples
}

func summarize(samples []parsedSample) []benchmarkResult {
	grouped := make(map[string][]benchmarkSample)
	for _, sample := range samples {
		grouped[sample.name] = append(grouped[sample.name], sample.sample)
	}

	names := make([]string, 0, len(grouped))
	for name := range grouped {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]benchmarkResult, 0, len(names))
	for _, name := range names {
		resultSamples := grouped[name]
		results = append(results, benchmarkResult{
			Name:              name,
			Samples:           resultSamples,
			MedianNSPerOp:     median(resultSamples, func(sample benchmarkSample) (float64, bool) { return sample.NSPerOp, true }),
			MedianBytesPerOp:  medianPointer(resultSamples, func(sample benchmarkSample) (float64, bool) { return pointerValue(sample.BytesPerOp) }),
			MedianAllocsPerOp: medianPointer(resultSamples, func(sample benchmarkSample) (float64, bool) { return pointerValue(sample.AllocsPerOp) }),
		})
	}
	return results
}

func pointerValue(value *float64) (float64, bool) {
	if value == nil {
		return 0, false
	}
	return *value, true
}

func median(samples []benchmarkSample, selectValue func(benchmarkSample) (float64, bool)) float64 {
	values := selectedValues(samples, selectValue)
	if len(values) == 0 {
		return 0
	}
	return medianFloat(values)
}

func medianPointer(samples []benchmarkSample, selectValue func(benchmarkSample) (float64, bool)) *float64 {
	values := selectedValues(samples, selectValue)
	if len(values) == 0 {
		return nil
	}
	value := medianFloat(values)
	return &value
}

func selectedValues(samples []benchmarkSample, selectValue func(benchmarkSample) (float64, bool)) []float64 {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		if value, ok := selectValue(sample); ok {
			values = append(values, value)
		}
	}
	sort.Float64s(values)
	return values
}

func medianFloat(values []float64) float64 {
	if len(values)%2 == 1 {
		return values[len(values)/2]
	}
	middle := len(values) / 2
	return (values[middle-1] + values[middle]) / 2
}

func normalizeBenchmarkName(name string) string {
	if index := strings.LastIndex(name, "-"); index > 0 && allDigits(name[index+1:]) {
		return name[:index]
	}
	return name
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func writeRun(outDir, stamp, rawOutput string, run *storedRun) error {
	runDir := filepath.Join(outDir, "runs", run.System.ID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return err
	}

	rawRel := filepath.ToSlash(filepath.Join("runs", run.System.ID, stamp+".txt"))
	dataRel := filepath.ToSlash(filepath.Join("runs", run.System.ID, stamp+".json"))
	run.RawOutput = rawRel
	run.DataFile = dataRel

	if err := os.WriteFile(filepath.Join(outDir, rawRel), []byte(rawOutput), 0644); err != nil {
		return err
	}

	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(outDir, dataRel), data, 0644); err != nil {
		return err
	}

	return nil
}

func loadRuns(outDir string) ([]storedRun, error) {
	files, err := filepath.Glob(filepath.Join(outDir, "runs", "*", "*.json"))
	if err != nil {
		return nil, err
	}

	runs := make([]storedRun, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}

		var run storedRun
		if err := json.Unmarshal(data, &run); err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}

		rel, err := filepath.Rel(outDir, file)
		if err != nil {
			return nil, err
		}
		run.DataFile = filepath.ToSlash(rel)
		runs = append(runs, run)
	}

	sortRuns(runs)
	return runs, nil
}

func sortRuns(runs []storedRun) {
	sort.Slice(runs, func(i, j int) bool {
		left := parseTime(runs[i].Timestamp)
		right := parseTime(runs[j].Timestamp)
		if left.Equal(right) {
			return runs[i].DataFile < runs[j].DataFile
		}
		return left.Before(right)
	})
}

func findRun(runs []storedRun, dataFile string) *storedRun {
	for i := range runs {
		if runs[i].DataFile == dataFile {
			return &runs[i]
		}
	}
	return nil
}

func writeReport(outDir string, current storedRun, runs []storedRun) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	previous := previousSameHardware(current, runs)
	latestBySystem := latestRunsBySystem(runs)

	var b strings.Builder
	b.WriteString("# Benchmark Report\n\n")
	b.WriteString("Generated by `make bench`.\n\n")
	b.WriteString("Comparisons are only made between runs with identical `goos`, `goarch`, and `cpu` values. Runs from different hardware are listed as separate systems and are not compared against each other.\n\n")

	b.WriteString("## Current Run\n\n")
	writeRunSummary(&b, current)

	b.WriteString("## Current vs Previous Same Hardware\n\n")
	if previous == nil {
		b.WriteString("No previous run with matching hardware was found. This run is the baseline for this system.\n\n")
	} else {
		fmt.Fprintf(&b, "Previous run: [%s](%s) from `%s`.\n\n", previous.DataFile, previous.DataFile, previous.Timestamp)
		writeComparisonTable(&b, *previous, current)
	}

	b.WriteString("## Latest Results By System\n\n")
	if len(latestBySystem) == 0 {
		b.WriteString("No benchmark runs have been stored yet.\n")
	} else {
		for _, run := range latestBySystem {
			fmt.Fprintf(&b, "### %s\n\n", run.System.ID)
			writeRunSummary(&b, run)
			writeResultsTable(&b, run.Results)
		}
	}

	return os.WriteFile(filepath.Join(outDir, "report.md"), []byte(b.String()), 0644)
}

func writeRunSummary(b *strings.Builder, run storedRun) {
	fmt.Fprintf(b, "- Timestamp: `%s`\n", run.Timestamp)
	fmt.Fprintf(b, "- System: `%s/%s`, CPU `%s`\n", run.System.GOOS, run.System.GOARCH, run.System.CPU)
	if run.GoVersion != "" {
		fmt.Fprintf(b, "- Go: `%s`\n", run.GoVersion)
	}
	if run.GitCommit != "" {
		fmt.Fprintf(b, "- Git commit: `%s`\n", run.GitCommit)
	}
	if run.GitDirty != nil {
		status := "clean"
		if *run.GitDirty {
			status = "dirty"
		}
		fmt.Fprintf(b, "- Git tree: `%s`\n", status)
	}
	fmt.Fprintf(b, "- Command: `%s`\n", strings.Join(run.Command, " "))
	fmt.Fprintf(b, "- Raw output: [%s](%s)\n", run.RawOutput, run.RawOutput)
	fmt.Fprintf(b, "- Parsed data: [%s](%s)\n\n", run.DataFile, run.DataFile)
}

func writeComparisonTable(b *strings.Builder, previous, current storedRun) {
	previousResults := resultMap(previous.Results)
	b.WriteString("| Benchmark | Previous ns/op | Current ns/op | ns/op change | Current B/op | Current allocs/op |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, currentResult := range current.Results {
		previousResult, ok := previousResults[currentResult.Name]
		if !ok {
			fmt.Fprintf(b, "| `%s` | n/a | %s | n/a | %s | %s |\n",
				currentResult.Name,
				formatNumber(currentResult.MedianNSPerOp),
				formatOptional(currentResult.MedianBytesPerOp),
				formatOptional(currentResult.MedianAllocsPerOp),
			)
			continue
		}
		fmt.Fprintf(b, "| `%s` | %s | %s | %s | %s | %s |\n",
			currentResult.Name,
			formatNumber(previousResult.MedianNSPerOp),
			formatNumber(currentResult.MedianNSPerOp),
			formatDelta(previousResult.MedianNSPerOp, currentResult.MedianNSPerOp),
			formatOptional(currentResult.MedianBytesPerOp),
			formatOptional(currentResult.MedianAllocsPerOp),
		)
	}
	b.WriteString("\n")
}

func writeResultsTable(b *strings.Builder, results []benchmarkResult) {
	b.WriteString("| Benchmark | Median ns/op | Median B/op | Median allocs/op |\n")
	b.WriteString("| --- | ---: | ---: | ---: |\n")
	for _, result := range results {
		fmt.Fprintf(b, "| `%s` | %s | %s | %s |\n",
			result.Name,
			formatNumber(result.MedianNSPerOp),
			formatOptional(result.MedianBytesPerOp),
			formatOptional(result.MedianAllocsPerOp),
		)
	}
	b.WriteString("\n")
}

func resultMap(results []benchmarkResult) map[string]benchmarkResult {
	mapped := make(map[string]benchmarkResult, len(results))
	for _, result := range results {
		mapped[result.Name] = result
	}
	return mapped
}

func previousSameHardware(current storedRun, runs []storedRun) *storedRun {
	var previous *storedRun
	currentTime := parseTime(current.Timestamp)
	for i := range runs {
		run := &runs[i]
		if run.DataFile == current.DataFile || !sameHardware(current.System, run.System) {
			continue
		}
		runTime := parseTime(run.Timestamp)
		if !runTime.Before(currentTime) {
			continue
		}
		if previous == nil || runTime.After(parseTime(previous.Timestamp)) {
			previous = run
		}
	}
	return previous
}

func sameHardware(left, right systemInfo) bool {
	return left.GOOS == right.GOOS && left.GOARCH == right.GOARCH && left.CPU == right.CPU
}

func latestRunsBySystem(runs []storedRun) []storedRun {
	latest := make(map[string]storedRun)
	for _, run := range runs {
		existing, ok := latest[run.System.ID]
		if !ok || parseTime(run.Timestamp).After(parseTime(existing.Timestamp)) {
			latest[run.System.ID] = run
		}
	}

	result := make([]storedRun, 0, len(latest))
	for _, run := range latest {
		result = append(result, run)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].System.ID < result[j].System.ID
	})
	return result
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func formatDelta(previous, current float64) string {
	if previous == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%+.2f%%", ((current-previous)/previous)*100)
}

func formatOptional(value *float64) string {
	if value == nil {
		return "n/a"
	}
	return formatNumber(*value)
}

func formatNumber(value float64) string {
	formatted := strconv.FormatFloat(value, 'f', 2, 64)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	return formatted
}

func slug(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
