package mykv

import (
	"fmt"
	"os"
	"sync"
)

type DB struct {
	writeLock  sync.Mutex   // Allows only one writer at a time.
	commitLock sync.RWMutex // Keeps readers away from the short disk-apply phase.
	*dal                    // Data Access Layer
}

// Open opens a database at the given path with the provided options.
func Open(path string, options *Options) (*DB, error) {
	if options == nil {
		options = DefaultOptions
	}
	options = options.withDefaults()
	if err := options.validate(); err != nil {
		return nil, err
	}
	dal, err := newDal(path, options)
	if err != nil {
		return nil, err
	}
	db := &DB{
		dal: dal,
	}
	return db, nil
}

func (o *Options) withDefaults() *Options {
	options := *o
	if options.pageSize == 0 {
		options.pageSize = os.Getpagesize()
	}
	if options.MinFillPercent == 0 {
		options.MinFillPercent = DefaultOptions.MinFillPercent
	}
	if options.MaxFillPercent == 0 {
		options.MaxFillPercent = DefaultOptions.MaxFillPercent
	}
	return &options
}

func (o *Options) validate() error {
	if o.pageSize <= 0 {
		return fmt.Errorf("%w: page size must be positive", errInvalidOptions)
	}
	if o.MinFillPercent <= 0 || o.MinFillPercent >= 1 {
		return fmt.Errorf("%w: min fill percent must be greater than 0 and less than 1", errInvalidOptions)
	}
	if o.MaxFillPercent <= 0 || o.MaxFillPercent > 1 {
		return fmt.Errorf("%w: max fill percent must be greater than 0 and less than or equal to 1", errInvalidOptions)
	}
	if o.MinFillPercent >= o.MaxFillPercent {
		return fmt.Errorf("%w: min fill percent must be less than max fill percent", errInvalidOptions)
	}
	if o.pageSize < metaHeaderSize() {
		return fmt.Errorf("%w: page size must fit metadata header", errInvalidOptions)
	}
	return nil
}

// Close closes the database.
func (db *DB) Close() error {
	return db.close()
}

// ReadTx starts a read-only transaction.
func (db *DB) ReadTx() *Tx {
	db.commitLock.RLock()
	return newTx(db, false)
}

// WriteTx starts a read-write transaction.
func (db *DB) WriteTx() *Tx {
	db.writeLock.Lock()
	return newTx(db, true)
}
