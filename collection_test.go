package mykv

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_GetAndCreateCollection tests getting and creating a collection.
func Test_GetAndCreateCollection(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx := db.WriteTx()

	collectionName := testCollectionName
	createdCollection, err := tx.CreateCollection(collectionName)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	tx = db.ReadTx()
	actual, err := tx.GetCollection(collectionName)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	expected := newEmptyCollection()
	expected.root = createdCollection.root
	expected.counter = 0
	expected.name = collectionName

	areCollectionsEqual(t, expected, actual)
}

// Test_GetCollectionDoesntExist tests getting a collection that doesn't exist.
func Test_GetCollectionDoesntExist(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx := db.ReadTx()
	collection, err := tx.GetCollection([]byte("name1"))
	require.NoError(t, err)

	assert.Nil(t, collection)
}

// Test_CreateCollectionPutItem tests creating a collection and putting an item in it.
func Test_CreateCollectionPutItem(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

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
}

func TestPutRejectsOversizedValue(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx := db.WriteTx()
	collection, err := tx.CreateCollection(testCollectionName)
	require.NoError(t, err)

	err = collection.Put([]byte("key"), make([]byte, 256))
	require.ErrorIs(t, err, errItemTooLarge)

	tx.Rollback()
}

// Test_DeleteCollection tests deleting a collection.
func Test_DeleteCollection(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

	tx := db.WriteTx()

	collectionName := testCollectionName
	createdCollection, err := tx.CreateCollection(collectionName)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	tx = db.WriteTx()
	actual, err := tx.GetCollection(collectionName)
	require.NoError(t, err)

	areCollectionsEqual(t, createdCollection, actual)

	err = tx.DeleteCollection(createdCollection.name)
	require.NoError(t, err)

	actualAfterRemoval, err := tx.GetCollection(collectionName)
	require.NoError(t, err)
	assert.Nil(t, actualAfterRemoval)

	err = tx.Commit()
	require.NoError(t, err)
}

// Test_DeleteItem tests deleting an item from a collection.
func Test_DeleteItem(t *testing.T) {
	db, cleanFunc := createTestDB(t)
	defer cleanFunc()

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
}

// TestSerializeCollection tests serializing a collection.
func TestSerializeCollection(t *testing.T) {
	expectedCollectionValue, err := os.ReadFile(getExpectedResultFileName(t.Name()))
	require.NoError(t, err)

	expected := &Item{
		key:   []byte("collection1"),
		value: expectedCollectionValue,
	}

	collection := &Collection{
		name:    []byte("collection1"),
		root:    1,
		counter: 1,
	}

	actual := collection.serialize()
	assert.Equal(t, expected, actual)
}

// TestDeserializeCollection tests deserializing a collection.
func TestDeserializeCollection(t *testing.T) {
	expectedCollectionValue, err := os.ReadFile(getExpectedResultFileName(t.Name()))

	expected := &Collection{
		name:    []byte("collection1"),
		root:    1,
		counter: 1,
	}

	collection := &Item{
		key:   []byte("collection1"),
		value: expectedCollectionValue,
	}
	actual := newEmptyCollection()
	actual.deserialize(collection)

	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}
