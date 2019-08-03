package encoding

import (
	"fmt"

	"github.com/eleme/lindb/pkg/stream"
	"github.com/eleme/lindb/pkg/util"
)

// IntPackingEncoder represents median based delta storage.
// Uint32 array must keep in order
type IntPackingEncoder struct {
	values   []uint32
	size     int    // the size of the array (the number of elements it contains).
	previous uint32 // previous value
}

// IntPackingDecoder represents read value based on subscript
type IntPackingDecoder struct {
	reader       *stream.ByteBufReader
	size         int
	midValue     uint32 // median
	deltaByteLen int
	pos          int
}

func NewIntPackingEncoder() *IntPackingEncoder {
	return &IntPackingEncoder{
		values: make([]uint32, 16),
	}
}

func NewIntPackingDecoder(byteArray []byte) *IntPackingDecoder {
	reader := stream.NewBufReader(byteArray)
	count := int(reader.ReadUvarint64())
	midValue := uint32(reader.ReadUvarint64())
	deltaByteLen := reader.ReadByte()

	return &IntPackingDecoder{
		reader:       reader,
		size:         count,
		midValue:     midValue,
		deltaByteLen: int(deltaByteLen),
		pos:          reader.GetPosition(),
	}
}

// ensureSize represents increases the capacity of this offset instance, if necessary.
func (i *IntPackingEncoder) ensureSize() {
	if i.size == len(i.values) {
		newValues := make([]uint32, len(i.values)*2)
		copy(newValues, i.values)
		i.values = newValues
	}
}

// Add represents appends an uint32 to the end of this array.
func (i *IntPackingEncoder) Add(v uint32) error {
	if v < i.previous {
		return fmt.Errorf("uint32 must keep in order")
	}
	i.ensureSize()

	i.values[i.size] = v
	i.size++
	i.previous = v
	return nil
}

// Bytes returns compress data for uint32 array
func (i *IntPackingEncoder) Bytes() ([]byte, error) {
	writer := stream.BinaryWriter()
	// write size
	writer.PutUvarint32(uint32(i.size))

	midIdx := int(i.size / 2)
	midValue := i.values[i.size/2]
	maxDelta := i.values[i.size-1] - midValue
	maxDelta = max(maxDelta, midValue-i.values[0])
	deltaLength := intOfBytes(maxDelta)

	// write median value
	writer.PutUvarint32(midValue)
	// write max delta byte length
	writer.PutByte(deltaLength)

	for idx, v := range i.values {
		if idx >= i.size {
			break
		}
		var delta uint32
		if idx <= midIdx {
			delta = midValue - v
		} else {
			delta = v - midValue
		}
		by := util.Uint32ToBytes(delta)
		writer.PutBytes(by[:deltaLength])
	}

	return writer.Bytes()
}

// Get returns value at the specified position in this array.
func (d *IntPackingDecoder) Get(idx int) (uint32, error) {
	if idx >= d.size {
		return 0, fmt.Errorf("exceeding array length,[%d > %d] ", idx, d.size)
	}

	d.reader.NewPosition(d.pos + idx*d.deltaByteLen)
	v := d.reader.ReadBytes(d.deltaByteLen)

	var id uint32
	for i := 0; i < len(v); i++ {
		id |= uint32(v[i]) << uint32(8*i)
	}
	if idx <= d.size/2 {
		return d.midValue - id, nil
	} else {
		return d.midValue + id, nil
	}
}

// intOfBytes returns uint32 valid byte length
func intOfBytes(v uint32) (byteLength byte) {
	if v < 1<<8 {
		byteLength = 1
	} else if v < 1<<16 {
		byteLength = 2
	} else if v < 1<<24 {
		byteLength = 3
	} else {
		byteLength = 4
	}
	return byteLength
}

func max(x, y uint32) uint32 {
	if x < y {
		return y
	}
	return x
}
