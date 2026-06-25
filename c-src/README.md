# C Utilities

This directory contains small native utilities used while developing and inspecting `mykv`.

## `check_cache.c`

`check_cache.c` reports whether pages from a file are currently resident in the operating system page cache.

It is useful when experimenting with database file reads and writes, especially when checking whether a `data.db` page has already been loaded by the OS.

### How It Works

The program:

1. Opens the file passed on the command line.
2. Uses `lseek` to determine the file size.
3. Maps the file with `mmap(..., PROT_NONE, MAP_SHARED, ...)`.
4. Allocates one byte per OS page.
5. Calls `mincore` to ask the kernel which mapped pages are resident.
6. Prints one line per page:

```text
Page 0: in cache
Page 1: not in cache
```

The mapping uses `PROT_NONE`, so the program does not read the file contents itself. It only asks the kernel about residency.

### Build

From the repository root:

```sh
make c-tools
```

Or directly:

```sh
cc -Wall -Wextra -O2 -o c-src/check_cache c-src/check_cache.c
```

### Run

```sh
c-src/check_cache data.db
```

### Platform Notes

This utility depends on POSIX APIs:

- `open`
- `lseek`
- `mmap`
- `mincore`
- `sysconf`

It is intended for Unix-like systems such as macOS and Linux. `mincore` behavior and permissions can vary across platforms.

### Limitations

- Empty files cannot be meaningfully mapped.
- Output is based on OS page size, not the database page size.
- Page cache residency can change immediately after the program runs.
- It does not validate that the file is a `mykv` database.
