package mykv

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFreelistSerialize tests the serialization of the freelist.
func TestFreelistSerialize(t *testing.T) {
	freelist := newFreelist()
	freelist.maxPage = 5
	freelist.releasedPages = []pgnum{1, 2, 3}
	actual := freelist.serialize(make([]byte, testPageSize))

	expected, err := os.ReadFile(getExpectedResultFileName(t.Name()))
	require.NoError(t, err)

	assert.Equal(t, expected, actual)
}

// TestFreelistDeserialize tests the deserialization of the freelist.
func TestFreelistDeserialize(t *testing.T) {
	freelist, err := os.ReadFile(getExpectedResultFileName(t.Name()))
	actual := newFreelist()
	actual.deserialize(freelist)
	require.NoError(t, err)

	expected := newFreelist()
	expected.maxPage = 5
	expected.releasedPages = []pgnum{1, 2, 3}

	assert.Equal(t, expected, actual)
}
