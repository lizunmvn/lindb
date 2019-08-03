package index

import (
	"github.com/vmihailenco/msgpack"
	"math"
)

const (
	NotFoundTagsID uint32 = math.MaxInt32

	NotFoundMetricID uint32 = math.MaxUint32

	MetricSequenceIDKey = 0

	NotFoundFieldID uint32 = math.MaxUint32
)

func BytesToMap(tags []byte) (tagsMap map[string]string, err error) {
	err = msgpack.Unmarshal(tags, &tagsMap)
	return tagsMap, err
}

func MapToBytes(tagsMap map[string]string) ([]byte, error) {
	return msgpack.Marshal(tagsMap)
}
