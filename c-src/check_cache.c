#include <sys/mman.h>
#include <fcntl.h>
#include <stdio.h>
#include <unistd.h>
#include <stdlib.h>
#include <stdint.h>

/*

Check if the file is in memory

*/

int main(int argc, char *argv[]) {
    if (argc != 2) {
        fprintf(stderr, "Usage: %s <file>\n", argv[0]);
        return 1;
    }

    int fd = open(argv[1], O_RDONLY);
    if (fd == -1) {
        perror("open");
        return 1;
    }

    off_t size = lseek(fd, 0, SEEK_END);
    if (size == -1) {
        perror("lseek");
        close(fd);
        return 1;
    }

    void *addr = mmap(NULL, size, PROT_NONE, MAP_SHARED, fd, 0);
    if (addr == MAP_FAILED) {
        perror("mmap");
        close(fd);
        return 1;
    }

    char *vec = calloc(1, (size + sysconf(_SC_PAGESIZE) - 1) / sysconf(_SC_PAGESIZE));
    if (!vec) {
        perror("calloc");
        munmap(addr, size);
        close(fd);
        return 1;
    }

    if (mincore(addr, size, vec) == -1) {
        perror("mincore");
        free(vec);
        munmap(addr, size);
        close(fd);
        return 1;
    }

    for (off_t i = 0; i < size / sysconf(_SC_PAGESIZE); i++) {
        printf("Page %jd: %s\n", (intmax_t)i, vec[i] & 1 ? "in cache" : "not in cache");
    }

    free(vec);
    munmap(addr, size);
    close(fd);
    return 0;
}
