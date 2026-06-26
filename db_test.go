package mykv

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDB_CreateCollectionPutItem tests creating a collection and putting an item in it.
func TestDB_CreateCollectionPutItem(t *testing.T) {
	db, err := Open(getTempFileName(), &Options{MinFillPercent: 0.5, MaxFillPercent: 1.0})
	require.NoError(t, err)

	tx := db.WriteTx()
	collectionName := testCollectionName
	createdCollection, err := tx.CreateCollection(collectionName)
	require.NoError(t, err)

	newKey := []byte("0")
	newVal := []byte("1")
	err = createdCollection.Put(newKey, newVal)
	require.NoError(t, err)

	item, err := createdCollection.Find(newKey)
	require.NoError(t, err)

	assert.Equal(t, newKey, item.key)
	assert.Equal(t, newVal, item.value)

	err = tx.Commit()
	require.NoError(t, err)
}

func TestOpenWithInvalidOptions(t *testing.T) {
	_, err := Open(getTempFileName(), &Options{MinFillPercent: 0.9, MaxFillPercent: 0.5})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errInvalidOptions))
}

// TestDB_WritersDontBlockReaders tests that writers do not block readers.
func TestDB_WritersDontBlockReaders(t *testing.T) {
	db, err := Open(getTempFileName(), &Options{MinFillPercent: 0.5, MaxFillPercent: 1.0})
	require.NoError(t, err)

	tx := db.WriteTx()
	collectionName := testCollectionName
	createdCollection, err := tx.CreateCollection(collectionName)
	require.NoError(t, err)

	newKey := []byte("0")
	newVal := []byte("1")
	err = createdCollection.Put(newKey, newVal)
	require.NoError(t, err)

	item, err := createdCollection.Find(newKey)
	require.NoError(t, err)

	assert.Equal(t, newKey, item.key)
	assert.Equal(t, newVal, item.value)

	err = tx.Commit()
	require.NoError(t, err)

	// Now open a write tx and try to read while that tx is open
	holdingTx := db.WriteTx()

	readTx := db.ReadTx()

	collection, err := readTx.GetCollection(createdCollection.name)
	areCollectionsEqual(t, createdCollection, collection)

	err = readTx.Commit()
	require.NoError(t, err)

	err = holdingTx.Commit()
	require.NoError(t, err)
}

// TestDB_ReadersDontSeeUncommittedChanges tests that readers do not see uncommitted changes.
func TestDB_ReadersDontSeeUncommittedChanges(t *testing.T) {
	db, err := Open(getTempFileName(), &Options{MinFillPercent: 0.5, MaxFillPercent: 1.0})
	require.NoError(t, err)

	tx := db.WriteTx()
	collectionName := testCollectionName
	createdCollection, err := tx.CreateCollection(collectionName)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	tx2 := db.WriteTx()
	createdCollection, err = tx2.GetCollection(createdCollection.name)
	require.NoError(t, err)

	newKey := createItem("0")
	newVal := createItem("1")
	err = createdCollection.Put(newKey, newVal)
	require.NoError(t, err)

	readTx := db.ReadTx()

	collection, err := readTx.GetCollection(createdCollection.name)
	areCollectionsEqual(t, createdCollection, collection)

	item, err := collection.Find(newKey)
	require.NoError(t, err)
	assert.Nil(t, item)

	err = readTx.Commit()
	require.NoError(t, err)

	err = tx2.Commit()
	require.NoError(t, err)
}

// TestDB_DeleteItem tests deleting an item from a collection.
func TestDB_DeleteItem(t *testing.T) {
	db, err := Open(getTempFileName(), &Options{MinFillPercent: testMinPercentage, MaxFillPercent: testMaxPercentage})
	require.NoError(t, err)

	tx := db.WriteTx()

	collectionName := testCollectionName
	createdCollection, err := tx.CreateCollection(collectionName)
	require.NoError(t, err)

	newKey := []byte("0")
	newVal := []byte("1")
	err = createdCollection.Put(newKey, newVal)
	require.NoError(t, err)

	item, err := createdCollection.Find(newKey)
	require.NoError(t, err)

	assert.Equal(t, newKey, item.key)
	assert.Equal(t, newVal, item.value)

	err = createdCollection.Remove(item.key)
	require.NoError(t, err)

	item, err = createdCollection.Find(newKey)
	require.NoError(t, err)

	assert.Nil(t, item)

	err = tx.Commit()
	require.NoError(t, err)
}

func TestDB_ReplaysWALOnOpen(t *testing.T) {
	path := getTempFileName()
	db, err := Open(path, &Options{pageSize: testPageSize, MinFillPercent: testMinPercentage, MaxFillPercent: testMaxPercentage})
	require.NoError(t, err)

	tx := db.WriteTx()
	collection, err := tx.CreateCollection(testCollectionName)
	require.NoError(t, err)
	require.NoError(t, collection.Put([]byte("key"), []byte("value")))
	require.NoError(t, tx.flushCollectionMetadata())
	pages, _, _, err := tx.commitPages()
	require.NoError(t, err)
	require.NoError(t, db.writeWAL(pages))
	db.writeLock.Unlock()
	require.NoError(t, db.Close())

	db, err = Open(path, &Options{pageSize: testPageSize, MinFillPercent: testMinPercentage, MaxFillPercent: testMaxPercentage})
	require.NoError(t, err)
	defer db.Close()

	readTx := db.ReadTx()
	actualCollection, err := readTx.GetCollection(testCollectionName)
	require.NoError(t, err)
	require.NotNil(t, actualCollection)
	item, err := actualCollection.Find([]byte("key"))
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, []byte("value"), item.Value())
	require.NoError(t, readTx.Commit())

	_, err = os.Stat(path + ".wal")
	require.True(t, os.IsNotExist(err))
}

func TestDB_CollectionRootSplitPersistsAfterReopen(t *testing.T) {
	path := getTempFileName()
	options := &Options{pageSize: testPageSize, MinFillPercent: testMinPercentage, MaxFillPercent: testMaxPercentage}
	db, err := Open(path, options)
	require.NoError(t, err)

	tx := db.WriteTx()
	collection, err := tx.CreateCollection(testCollectionName)
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		key := createItem(string(rune('a' + i)))
		require.NoError(t, collection.Put(key, key))
	}
	require.NoError(t, tx.Commit())
	require.NoError(t, db.Close())

	db, err = Open(path, options)
	require.NoError(t, err)
	defer db.Close()

	readTx := db.ReadTx()
	collection, err = readTx.GetCollection(testCollectionName)
	require.NoError(t, err)
	require.NotNil(t, collection)
	for i := 0; i < 20; i++ {
		key := createItem(string(rune('a' + i)))
		item, err := collection.Find(key)
		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, key, item.Value())
	}
	require.NoError(t, readTx.Commit())
}

func TestDB_ReusesTrackedCollectionHandleInWriteTx(t *testing.T) {
	path := getTempFileName()
	options := &Options{pageSize: testPageSize, MinFillPercent: testMinPercentage, MaxFillPercent: testMaxPercentage}
	db, err := Open(path, options)
	require.NoError(t, err)

	tx := db.WriteTx()
	collection, err := tx.CreateCollection(testCollectionName)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	tx = db.WriteTx()
	collection, err = tx.GetCollection(testCollectionName)
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		key := createItem(string(rune('a' + i)))
		require.NoError(t, collection.Put(key, key))
	}
	again, err := tx.GetCollection(testCollectionName)
	require.NoError(t, err)
	require.Same(t, collection, again)
	require.NoError(t, tx.Commit())
	require.NoError(t, db.Close())

	db, err = Open(path, options)
	require.NoError(t, err)
	defer db.Close()

	readTx := db.ReadTx()
	collection, err = readTx.GetCollection(testCollectionName)
	require.NoError(t, err)
	require.NotNil(t, collection)
	item, err := collection.Find(createItem("t"))
	require.NoError(t, err)
	require.NotNil(t, item)
	require.NoError(t, readTx.Commit())
}

func TestTx_CommitErrorReleasesWriteLock(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx := db.WriteTx()
	collection, err := tx.CreateCollection(testCollectionName)
	require.NoError(t, err)
	require.NoError(t, collection.Put([]byte("key"), []byte("value")))

	require.NoError(t, db.file.Close())
	err = tx.Commit()
	require.Error(t, err)

	done := make(chan struct{})
	go func() {
		nextTx := db.WriteTx()
		nextTx.Rollback()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("write lock was not released after failed commit")
	}
}
