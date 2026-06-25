package mykv

import "errors"

const (
	magicNumberSize = 4  // magicNumberSize represents the size of the magic number in bytes
	versionSize     = 4  // versionSize represents the size of the file format version in bytes
	counterSize     = 8  // counterSize represents the size of the counter in bytes
	nodeHeaderSize  = 3  // nodeHeaderSize represents the size of the node header in bytes
	collectionSize  = 16 // collectionSize represents the size of the collection in bytes
	pageNumSize     = 8  // pageNumSize represents the size of the page number in bytes
)

// errWriteInsideReadTx is an error indicating that a write operation was attempted inside a read transaction
var errWriteInsideReadTx = errors.New("can't perform a write operation inside a read transaction")

var (
	errInvalidOptions = errors.New("invalid options")
	errInvalidDBFile  = errors.New("invalid db file")
	errItemTooLarge   = errors.New("item is too large to fit in a page")
)
