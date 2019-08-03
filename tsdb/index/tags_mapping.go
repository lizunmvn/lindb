package index

import (
	"bytes"
	"fmt"

	"github.com/RoaringBitmap/roaring"

	"github.com/eleme/lindb/pkg/encoding"
	"github.com/eleme/lindb/pkg/stream"
)

// TagsMapping represents ID to tags mapping under for a measurement.
type TagsMapping struct {
	dictionary       map[string]uint32    // key is string, value is id
	dictionaryWriter *stream.Binary       // dictionary stream
	tagBlocks        map[string]*TagBlock // key is a tags combination key
}

// TagBlock represents a combining key data block
type TagBlock struct {
	keys       []uint32        // tag key ids
	bitmap     *roaring.Bitmap // tagID bitmap
	tagsWriter *stream.Binary
	intPacking *encoding.IntPackingEncoder
}

// TagsMappingReader represents tags mapping byte arrays for reading
type TagsMappingReader struct {
	reader    *stream.ByteBufReader //byte buf reader
	dicPos    int                   // dictionary start position
	tagBlocks map[*TagBlockReader]int
}

// TagBlockReader represents a tag mapping byte arrays for reading
type TagBlockReader struct {
	keys              []string
	reader            *stream.ByteBufReader //byte buf reader
	bitmap            *roaring.Bitmap       // tagID bitmap
	intPackingDecoder *encoding.IntPackingDecoder
	pos               int //tag value ids start position
}

func NewTagsMapping() *TagsMapping {
	return &TagsMapping{
		dictionary:       make(map[string]uint32),
		dictionaryWriter: stream.BinaryWriter(),
		tagBlocks:        make(map[string]*TagBlock),
	}
}

// newTagBlock returns a tag data block
func newTagBlock(keyIds []uint32) *TagBlock {
	return &TagBlock{
		keys:       keyIds,
		bitmap:     roaring.New(),
		tagsWriter: stream.BinaryWriter(),
		intPacking: encoding.NewIntPackingEncoder(),
	}
}

// newTagBlockReader returns a tag data block for reading
func newTagBlockReader(keys []string, byteArray []byte) (*TagBlockReader, error) {
	reader := stream.NewBufReader(byteArray)
	//reader bitmap
	_, b := reader.ReadLenBytes()
	bitmap := roaring.New()
	_, err := bitmap.ReadFrom(bytes.NewBuffer(b))
	if nil != err {
		return nil, err
	}

	_, intPackingBytes := reader.ReadLenBytes()
	intPackingDecoder := encoding.NewIntPackingDecoder(intPackingBytes)

	return &TagBlockReader{
		keys:              keys,
		reader:            reader,
		bitmap:            bitmap,
		intPackingDecoder: intPackingDecoder,
		pos:               reader.GetPosition(),
	}, nil
}

func NewTagsMappingReader(byteArray []byte) (*TagsMappingReader, error) {
	reader := stream.NewBufReader(byteArray)
	dicLen := int(reader.ReadUvarint64())
	dicPos := reader.GetPosition()
	reader.NewPosition(reader.GetPosition() + dicLen)
	count := int(reader.ReadUvarint64())

	tagBlocks := make(map[*TagBlockReader]int, count)
	for i := 0; i < count; i++ {
		keyCount := int(reader.ReadUvarint64())
		keys := make([]string, keyCount)
		for k := 0; k < keyCount; k++ {
			kPos := int(reader.ReadUvarint64())
			clone := reader.Duplicator()
			clone.NewPosition(dicPos + kPos)
			_, key := clone.ReadLenBytes()
			keys[k] = string(key)
		}

		_, tagBlockBytes := reader.ReadLenBytes()
		tagReader, err := newTagBlockReader(keys, tagBlockBytes)
		if nil != err {
			return nil, err
		}
		tagBlocks[tagReader] = keyCount
	}

	return &TagsMappingReader{
		reader:    reader,
		dicPos:    dicPos,
		tagBlocks: tagBlocks,
	}, nil
}

// getID returns id for given string key
func (t *TagsMapping) getID(key string) uint32 {
	id, ok := t.dictionary[key]
	if !ok {
		id = uint32(t.dictionaryWriter.Len())
		t.dictionaryWriter.PutLenBytes([]byte(key))
		t.dictionary[key] = id
	}
	return id
}

// Add represents add a tags mapping relationship,keys are ordered.
func (t *TagsMapping) Add(keys []string, values []string, tagsID uint32) error {
	if len(keys) != len(values) {
		return fmt.Errorf("a pair key-values length is inconsistent")
	}

	keyIds := make([]uint32, len(keys))
	for idx, key := range keys {
		keyIds[idx] = t.getID(key)
	}

	tag := keysToString(keyIds, "#")

	tagBlock, ok := t.tagBlocks[tag]
	if !ok {
		tagBlock = newTagBlock(keyIds)
		t.tagBlocks[tag] = tagBlock
	}

	return tagBlock.add(tagsID, values, t)
}

// add represents add value combination to TagBlock
func (b *TagBlock) add(tagsID uint32, values []string, t *TagsMapping) error {
	if b.bitmap.Contains(tagsID) {
		return fmt.Errorf("already contains the tagsID")
	}

	err := b.intPacking.Add(uint32(b.tagsWriter.Len()))
	if nil != err {
		return err
	}
	b.bitmap.Add(tagsID)

	for _, value := range values {
		id := t.getID(value)
		b.tagsWriter.PutUvarint32(id)
	}
	return nil
}

// GetTags returns tags string by tags ID
func (t *TagsMappingReader) GetTags(tagID uint32) (map[string]string, error) {
	for tb, count := range t.tagBlocks {
		idx := uint64(tb.bitmap.Rank(tagID))
		if idx > 0 {
			//roaring bitmap rank starting from 1
			idx--
			offset, err := tb.intPackingDecoder.Get(int(idx))
			if nil != err {
				return nil, err
			}

			tb.reader.NewPosition(tb.pos + int(offset))

			result := make(map[string]string, count)
			for i := 0; i < count; i++ {
				keyPos := int(tb.reader.ReadUvarint64())
				t.reader.NewPosition(t.dicPos + keyPos)
				_, key := t.reader.ReadLenBytes()
				result[tb.keys[i]] = string(key)
			}
			return result, nil
		}
	}
	return nil, nil
}

// Bytes returns compress data for tags mapping, if error return err
func (t *TagsMapping) Bytes() ([]byte, error) {
	writer := stream.BinaryWriter()
	dic, err := t.dictionaryWriter.Bytes()
	if nil != err {
		return nil, err
	}
	//write dic
	writer.PutLenBytes(dic)
	//write mapping count
	writer.PutUvarint32(uint32(len(t.tagBlocks)))
	for _, tagBlock := range t.tagBlocks {
		writer.PutUvarint32(uint32(len(tagBlock.keys)))
		for _, k := range tagBlock.keys {
			writer.PutUvarint32(k)
		}
		tagBlock.bitmap.RunOptimize()
		bitmapBytes, err := tagBlock.bitmap.ToBytes()
		if nil != err {
			return nil, err
		}

		mappingWriter := stream.BinaryWriter()
		mappingWriter.PutLenBytes(bitmapBytes)
		intBytes, _ := tagBlock.intPacking.Bytes()
		mappingWriter.PutLenBytes(intBytes)

		tagBytes, err := tagBlock.tagsWriter.Bytes()
		if nil != err {
			return nil, err
		}
		mappingWriter.PutBytes(tagBytes)

		data, err := mappingWriter.Bytes()
		if nil != err {
			return nil, err
		}
		writer.PutLenBytes(data)
	}

	return writer.Bytes()
}

func keysToString(ids []uint32, separator string) string {
	var buffer bytes.Buffer
	for i := 0; i < len(ids); i++ {
		buffer.WriteString(string(ids[i]))
		if i != len(ids)-1 {
			buffer.WriteString(separator)
		}
	}
	return buffer.String()
}
