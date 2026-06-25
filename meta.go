package mykv

import (
	"encoding/binary"
	"fmt"
)

const (
	magicNumber uint32 = 0xD00DB00D // data fil magix number
	fileVersion uint32 = 1
)

const (
	metaPageNum = 0 // meta is always in 1st page
)

// meta is the meta page of the db
type meta struct {
	// The database has a root collection that holds all the collections in the database.
	// It is called root and the root property of meta holds page number containing
	// the root of collections collection. The keys are the
	// collections names and the values are the page number of the root of each collection. Then, once the collection
	// and the root page are located, a search inside a collection can be made.
	root         pgnum
	freelistPage pgnum
}

// create new empty Meta
func newEmptyMeta() *meta {
	return &meta{}
}

func metaHeaderSize() int {
	return magicNumberSize + versionSize + pageNumSize + pageNumSize
}

// serialize meta block
func (m *meta) serialize(buf []byte) {
	pos := 0
	binary.LittleEndian.PutUint32(buf[pos:], magicNumber)
	pos += magicNumberSize

	binary.LittleEndian.PutUint32(buf[pos:], fileVersion)
	pos += versionSize

	binary.LittleEndian.PutUint64(buf[pos:], uint64(m.root))
	pos += pageNumSize

	binary.LittleEndian.PutUint64(buf[pos:], uint64(m.freelistPage))
	pos += pageNumSize
}

// deserialize meta data page
func (m *meta) deserialize(buf []byte) error {
	pos := 0
	magicNumberRes := binary.LittleEndian.Uint32(buf[pos:])
	pos += magicNumberSize

	if magicNumberRes != magicNumber {
		return fmt.Errorf("%w: incorrect magic number", errInvalidDBFile)
	}

	version := binary.LittleEndian.Uint32(buf[pos:])
	pos += versionSize
	if version != fileVersion {
		return fmt.Errorf("%w: unsupported file version %d", errInvalidDBFile, version)
	}

	m.root = pgnum(binary.LittleEndian.Uint64(buf[pos:]))
	pos += pageNumSize

	m.freelistPage = pgnum(binary.LittleEndian.Uint64(buf[pos:]))
	pos += pageNumSize
	return nil
}
