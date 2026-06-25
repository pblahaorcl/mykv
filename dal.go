package mykv

import (
	"errors"
	"fmt"
	"os"
)

type pgnum uint64

type Options struct {
	pageSize       int     // Block size
	MinFillPercent float32 // treshold for node merge
	MaxFillPercent float32 // node split treshold
}

// Default options
var DefaultOptions = &Options{
	MinFillPercent: 0.5,
	MaxFillPercent: 0.95,
}

// page represents disk page
type page struct {
	num  pgnum
	data []byte
}

type dal struct {
	pageSize       int
	minFillPercent float32
	maxFillPercent float32
	file           *os.File // data file
	*meta                   // metadata
	*freelist               // list of free pages
}

// newDal creates a new data access layer.
// newDal initializes a new data access layer (DAL) instance. It takes a file path and options as input.
// If the file at the given path exists, it reads the metadata and freelist from the file.
// If the file does not exist, it creates a new file, initializes the freelist, root, and writes the metadata.
// It returns a pointer to the initialized dal and an error if any operation fails.
//
// Parameters:
//   - path: The file path where the DAL data is stored.
//   - options: A pointer to Options struct containing configuration parameters.
//
// Returns:
//   - *dal: A pointer to the initialized dal instance.
//   - error: An error if any operation during initialization fails.
func newDal(path string, options *Options) (*dal, error) {
	dal := &dal{
		meta:           newEmptyMeta(),
		pageSize:       options.pageSize,
		minFillPercent: options.MinFillPercent,
		maxFillPercent: options.MaxFillPercent,
	}

	if _, err := os.Stat(path); err == nil { // exists the read from file
		dal.file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			_ = dal.close()
			return nil, err
		}

		meta, err := dal.readMeta()
		if err != nil {
			return nil, err
		}
		dal.meta = meta

		freelist, err := dal.readFreelist()
		if err != nil {
			return nil, err
		}
		dal.freelist = freelist
	} else if errors.Is(err, os.ErrNotExist) { // doesn't exist
		// init freelist
		dal.file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			_ = dal.close()
			return nil, err
		}

		dal.freelist = newFreelist()
		dal.freelistPage = dal.getNextPage()
		_, err := dal.writeFreelist()
		if err != nil {
			return nil, err
		}

		// init root
		collectionsNode, err := dal.writeNode(NewNodeForSerialization([]*Item{}, []pgnum{}))
		if err != nil {
			return nil, err
		}
		dal.root = collectionsNode.pageNum

		// write meta page
		if _, err = dal.writeMeta(dal.meta); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	return dal, nil
}

// getSplitIndex returns the index where a node should be split.
func (d *dal) getSplitIndex(node *Node) int {
	size := 0
	size += nodeHeaderSize

	for i := range node.items {
		size += node.elementSize(i)
		// if we have a big enough page size (more than minimum), and didn't reach the last node, which means we can
		// spare an element
		if float32(size) > d.minThreshold() && i < len(node.items)-1 {
			return i + 1
		}
	}
	return -1
}

// maxThreshold returns the maximum threshold for a node.
func (d *dal) maxThreshold() float32 {
	return d.maxFillPercent * float32(d.pageSize)
}

// isOverPopulated checks if a node is overpopulated.
func (d *dal) isOverPopulated(node *Node) bool {
	return float32(node.nodeSize()) > d.maxThreshold()
}

// minThreshold returns the minimum threshold for a node.
func (d *dal) minThreshold() float32 {
	return d.minFillPercent * float32(d.pageSize)
}

// isUnderPopulated checks if a node is underpopulated.
func (d *dal) isUnderPopulated(node *Node) bool {
	return float32(node.nodeSize()) < d.minThreshold()
}

// close closes the data access layer.
func (d *dal) close() error {
	if d.file != nil {
		err := d.file.Close()
		if err != nil {
			return fmt.Errorf("could not close file: %s", err)
		}
		d.file = nil
	}

	return nil
}

// allocateEmptyPage allocates an empty page.
func (d *dal) allocateEmptyPage() *page {
	return &page{
		data: make([]byte, d.pageSize),
	}
}

// readPage reads a page from the file.
func (d *dal) readPage(pageNum pgnum) (*page, error) {
	p := d.allocateEmptyPage()
	offset := int(pageNum) * d.pageSize
	_, err := d.file.ReadAt(p.data, int64(offset))
	if err != nil {
		return nil, err
	}
	return p, err
}

// writePage writes a page to the file.
func (d *dal) writePage(p *page) error {
	offset := int64(p.num) * int64(d.pageSize)
	_, err := d.file.WriteAt(p.data, offset)
	return err
}

// getNode retrieves a node from a page.
func (d *dal) getNode(pageNum pgnum) (*Node, error) {
	p, err := d.readPage(pageNum)
	if err != nil {
		return nil, err
	}
	node := NewEmptyNode()
	node.deserialize(p.data)
	if err := node.validate(d.pageSize); err != nil {
		return nil, err
	}
	node.pageNum = pageNum
	return node, nil
}

// writeNode writes a node to a page.
func (d *dal) writeNode(n *Node) (*Node, error) {
	if err := n.validate(d.pageSize); err != nil {
		return nil, err
	}
	p := d.allocateEmptyPage()
	if n.pageNum == 0 {
		p.num = d.getNextPage()
		n.pageNum = p.num
	} else {
		p.num = n.pageNum
	}
	p.data = n.serialize(p.data)
	err := d.writePage(p)
	if err != nil {
		return nil, err
	}
	return n, nil
}

// deleteNode deletes a node by releasing its page.
func (d *dal) deleteNode(pageNum pgnum) {
	d.releasePage(pageNum)
}

// readFreelist reads the freelist from the file.
func (d *dal) readFreelist() (*freelist, error) {
	p, err := d.readPage(d.freelistPage)
	if err != nil {
		return nil, err
	}

	freelist := newFreelist()
	freelist.deserialize(p.data)
	return freelist, nil
}

// writeFreelist writes the freelist to the file.
func (d *dal) writeFreelist() (*page, error) {
	p := d.allocateEmptyPage()
	p.num = d.freelistPage
	d.freelist.serialize(p.data)

	err := d.writePage(p)
	if err != nil {
		return nil, err
	}
	d.freelistPage = p.num
	return p, nil
}

// writeMeta writes the meta information to the file.
func (d *dal) writeMeta(meta *meta) (*page, error) {
	p := d.allocateEmptyPage()
	p.num = metaPageNum
	meta.serialize(p.data)

	err := d.writePage(p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// readMeta reads the meta information from the file.
func (d *dal) readMeta() (*meta, error) {
	p, err := d.readPage(metaPageNum)
	if err != nil {
		return nil, err
	}

	meta := newEmptyMeta()
	if err := meta.deserialize(p.data); err != nil {
		return nil, err
	}
	return meta, nil
}
