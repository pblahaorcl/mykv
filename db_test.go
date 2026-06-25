package mykv

import (
	"errors"
	"testing"

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
	t.Skip()
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
	t.Skip()
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
