CC ?= cc
CFLAGS ?= -Wall -Wextra -O2

BIN := check_cache
SRC := check_cache.c

all: $(BIN)

$(BIN): $(SRC)
	$(CC) $(CFLAGS) -o $@ $<

clean:
	rm -f $(BIN)

.PHONY: all clean
