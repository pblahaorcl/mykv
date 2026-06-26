package mykv

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

var walMagic = [8]byte{'M', 'Y', 'K', 'V', 'W', 'A', 'L', '1'}

const (
	walVersion    uint32 = 1
	walHeaderSize        = 8 + 4 + 4 + 8 + 4
)

func (d *dal) applyPagesWithWAL(pages []*page) error {
	if err := d.writeWAL(pages); err != nil {
		return err
	}
	if err := d.writePages(pages); err != nil {
		return err
	}
	if err := d.file.Sync(); err != nil {
		return err
	}
	return removeIfExists(d.walPath)
}

func (d *dal) recoverWAL() error {
	pages, ok, err := d.readWAL()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := d.writePages(pages); err != nil {
		return err
	}
	if err := d.file.Sync(); err != nil {
		return err
	}
	return removeIfExists(d.walPath)
}

func (d *dal) writeWAL(pages []*page) error {
	records := bytes.Buffer{}
	for _, p := range pages {
		if len(p.data) != d.pageSize {
			return fmt.Errorf("invalid WAL page size: got %d want %d", len(p.data), d.pageSize)
		}
		if err := binary.Write(&records, binary.LittleEndian, uint64(p.num)); err != nil {
			return err
		}
		if _, err := records.Write(p.data); err != nil {
			return err
		}
	}

	header := bytes.Buffer{}
	if _, err := header.Write(walMagic[:]); err != nil {
		return err
	}
	if err := binary.Write(&header, binary.LittleEndian, walVersion); err != nil {
		return err
	}
	if err := binary.Write(&header, binary.LittleEndian, uint32(d.pageSize)); err != nil {
		return err
	}
	if err := binary.Write(&header, binary.LittleEndian, uint64(len(pages))); err != nil {
		return err
	}
	if err := binary.Write(&header, binary.LittleEndian, crc32.ChecksumIEEE(records.Bytes())); err != nil {
		return err
	}

	f, err := os.OpenFile(d.walPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	if _, err = f.Write(header.Bytes()); err != nil {
		_ = f.Close()
		return err
	}
	if _, err = f.Write(records.Bytes()); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (d *dal) readWAL() ([]*page, bool, error) {
	data, err := os.ReadFile(d.walPath)
	if errorsIsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(data) == 0 {
		return nil, false, removeIfExists(d.walPath)
	}
	if len(data) < walHeaderSize {
		return nil, false, removeIfExists(d.walPath)
	}

	if !bytes.Equal(data[:8], walMagic[:]) {
		return nil, false, fmt.Errorf("invalid WAL magic")
	}
	pos := 8
	version := binary.LittleEndian.Uint32(data[pos:])
	pos += 4
	if version != walVersion {
		return nil, false, fmt.Errorf("unsupported WAL version %d", version)
	}
	pageSize := int(binary.LittleEndian.Uint32(data[pos:]))
	pos += 4
	if pageSize != d.pageSize {
		return nil, false, fmt.Errorf("WAL page size %d does not match database page size %d", pageSize, d.pageSize)
	}
	count := int(binary.LittleEndian.Uint64(data[pos:]))
	pos += 8
	checksum := binary.LittleEndian.Uint32(data[pos:])
	pos += 4

	recordSize := pageNumSize + pageSize
	expectedSize := walHeaderSize + count*recordSize
	if expectedSize != len(data) {
		return nil, false, removeIfExists(d.walPath)
	}
	records := data[pos:]
	if crc32.ChecksumIEEE(records) != checksum {
		return nil, false, fmt.Errorf("WAL checksum mismatch")
	}

	pages := make([]*page, 0, count)
	reader := bytes.NewReader(records)
	for i := 0; i < count; i++ {
		var pageNum uint64
		if err := binary.Read(reader, binary.LittleEndian, &pageNum); err != nil {
			return nil, false, err
		}
		p := &page{
			num:  pgnum(pageNum),
			data: make([]byte, pageSize),
		}
		if _, err := io.ReadFull(reader, p.data); err != nil {
			return nil, false, err
		}
		pages = append(pages, p)
	}
	return pages, true, nil
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errorsIsNotExist(err) {
		return err
	}
	return nil
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
