package mykv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTx_CreateCollection(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx := db.WriteTx()
	collection, err := tx.CreateCollection(testCollectionName)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	tx = db.ReadTx()
	actualCollection, err := tx.GetCollection(collection.name)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	areCollectionsEqual(t, collection, actualCollection)
}

func TestTx_CreateCollectionReadTx(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx := db.ReadTx()
	collection, err := tx.CreateCollection(testCollectionName)
	require.Error(t, err)
	require.Nil(t, collection)

	err = tx.Commit()
	require.NoError(t, err)
}

func TestTx_GetOrCreateCollection(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx := db.WriteTx()
	created, err := tx.GetOrCreateCollection(testCollectionName)
	require.NoError(t, err)
	require.NotNil(t, created)

	existing, err := tx.GetOrCreateCollection(testCollectionName)
	require.NoError(t, err)

	areCollectionsEqual(t, created, existing)
	require.NoError(t, tx.Commit())
}

func TestTx_OpenMultipleReadTxSimultaneously(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx1 := db.ReadTx()
	tx2 := db.ReadTx()

	collection1, err := tx1.GetCollection(testCollectionName)
	require.NoError(t, err)
	require.Nil(t, collection1)

	collection2, err := tx2.GetCollection(testCollectionName)
	require.NoError(t, err)
	require.Nil(t, collection2)

	err = tx1.Commit()
	require.NoError(t, err)
	err = tx2.Commit()
	require.NoError(t, err)
}

// TestTx_OpenReadAndWriteTxSimultaneously validates that read transactions can run while a writer is open, but they
// only observe committed data. The writer's commit waits for open readers before applying pages to disk.
func TestTx_OpenReadAndWriteTxSimultaneously(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx1 := db.ReadTx()

	tx2 := db.WriteTx()
	_, err := tx2.CreateCollection(testCollectionName)
	require.NoError(t, err)

	tx3 := db.ReadTx()
	collection3, err := tx3.GetCollection(testCollectionName)
	require.NoError(t, err)
	require.Nil(t, collection3)
	require.NoError(t, tx3.Commit())

	collection1, err := tx1.GetCollection(testCollectionName)
	require.NoError(t, err)
	require.Nil(t, collection1)

	commitDone := make(chan error, 1)
	go func() {
		commitDone <- tx2.Commit()
	}()

	select {
	case err := <-commitDone:
		require.NoError(t, err)
		t.Fatal("write commit completed while a read transaction was open")
	case <-time.After(50 * time.Millisecond):
	}

	require.NoError(t, tx1.Commit())
	require.NoError(t, <-commitDone)

	tx4 := db.ReadTx()
	collection4, err := tx4.GetCollection(testCollectionName)
	require.NoError(t, err)
	require.NotNil(t, collection4)
	require.NoError(t, tx4.Commit())
}

func TestTx_Rollback(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx := db.WriteTx()
	child0 := tx.writeNode(tx.newNode(createItems("0", "1", "2", "3"), []pgnum{}))

	child1 := tx.writeNode(tx.newNode(createItems("5", "6", "7", "8"), []pgnum{}))

	root := tx.writeNode(tx.newNode(createItems("4"), []pgnum{child0.pageNum, child1.pageNum}))

	collection, err := tx.createCollection(newCollection(testCollectionName, root.pageNum))
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	assert.Len(t, tx.db.freelist.releasedPages, 0)

	// Try to add 9 but then perform a rollback, so it won't be saved
	tx2 := db.WriteTx()

	collection, err = tx2.GetCollection(collection.name)
	require.NoError(t, err)

	val := createItem("9")
	err = collection.Put(val, val)
	require.NoError(t, err)

	tx2.Rollback()

	// 9 should not exist since a rollback was performed. Transaction-local page allocation means rollback does not
	// mutate the committed freelist.
	assert.Len(t, tx2.db.freelist.releasedPages, 0)
	tx3 := db.ReadTx()

	collection, err = tx3.GetCollection(collection.name)
	require.NoError(t, err)

	// Item not found
	expectedVal := createItem("9")
	item, err := collection.Find(expectedVal)
	require.NoError(t, err)
	assert.Nil(t, item)

	err = tx3.Commit()
	require.NoError(t, err)

	assert.Len(t, tx3.db.freelist.releasedPages, 0)
}
