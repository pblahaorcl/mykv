package mykv

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetaSerialize(t *testing.T) {
	meta := newEmptyMeta()
	meta.root = 3
	meta.freelistPage = 4
	actual := make([]byte, testPageSize)
	meta.serialize(actual)

	expected, err := os.ReadFile(getExpectedResultFileName(t.Name()))
	require.NoError(t, err)

	assert.Equal(t, expected, actual)
}

func TestCreateDalIncorrectMagicNumber(t *testing.T) {
	actualMetaBytes, err := os.ReadFile(getExpectedResultFileName(t.Name()))
	require.NoError(t, err)
	actualMeta := newEmptyMeta()
	err = actualMeta.deserialize(actualMetaBytes)
	require.ErrorIs(t, err, errInvalidDBFile)
}

func TestMetaDeserialize(t *testing.T) {
	actualMetaBytes, err := os.ReadFile(getExpectedResultFileName(t.Name()))
	require.NoError(t, err)
	actualMeta := newEmptyMeta()
	err = actualMeta.deserialize(actualMetaBytes)
	require.NoError(t, err)

	expectedMeta := newEmptyMeta()
	expectedMeta.root = 3
	expectedMeta.freelistPage = 4

	assert.Equal(t, expectedMeta, actualMeta)
}
