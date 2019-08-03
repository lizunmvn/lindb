package index

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagsMapping_Flush(t *testing.T) {
	tags := NewTagsMapping()

	keys := []string{"host", "dist", "partition"}
	hostCount := 4
	diskCount := 20
	partitionCount := 100
	id := 0
	for i := 0; i < hostCount; i++ {
		for j := 0; j < diskCount; j++ {
			for k := 0; k < partitionCount; k++ {
				id++
				var values []string
				values = append(values, fmt.Sprintf("%s%d", "host-", i))
				values = append(values, fmt.Sprintf("%s%d", "disk-", j))
				values = append(values, fmt.Sprintf("%s%d", "parition-", k))
				_ = tags.Add(keys, values, uint32(id))
			}
		}
	}

	by, _ := tags.Bytes()
	assert.Equal(t, hostCount*diskCount*partitionCount, id)
	reader, _ := NewTagsMappingReader(by)
	for i := 1; i <= id; i++ {
		tags, _ := reader.GetTags(uint32(i))
		assert.Equal(t, 3, len(tags))
	}
}
