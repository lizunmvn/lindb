package encoding

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestIntOffset_Add(t *testing.T) {
	intPacking := NewIntPackingEncoder()

	for i := 0; i < 100000; i++ {
		_ = intPacking.Add(uint32(i * 10))
	}

	by, _ := intPacking.Bytes()

	decoder := NewIntPackingDecoder(by)

	for i := 0; i < 100000+1; i++ {
		v, err := decoder.Get(i)
		if nil == err {
			assert.Equal(t, v, uint32(i*10))
		}
	}

}

func TestIntOffset_Add2(t *testing.T) {
	intOffset := NewIntPackingEncoder()
	var count = 1000

	for i := 0; i < count; i++ {
		_ = intOffset.Add(uint32(i * 3))
	}

	by, _ := intOffset.Bytes()

	decoder := NewIntPackingDecoder(by)

	for i := 0; i < count; i++ {
		v, _ := decoder.Get(i)
		assert.Equal(t, v, uint32(i*3))
	}
}

func TestIntOffset_AddRandom(t *testing.T) {
	intOffset := NewIntPackingEncoder()
	var count = 200
	var values []uint32
	for i := 0; i < count; i++ {
		v := uint32(rand.Intn(count * 2))
		values = append(values, v)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})

	for i := 0; i < count; i++ {
		err := intOffset.Add(values[i])
		if nil != err {
			fmt.Println(err)
		}
	}
	err := intOffset.Add(0)
	if nil != err {
		fmt.Println(err)
	}

	by, _ := intOffset.Bytes()

	decoder := NewIntPackingDecoder(by)
	fmt.Println(decoder.deltaByteLen)

	for i := 0; i < count; i++ {
		v, _ := decoder.Get(i)
		assert.Equal(t, v, values[i])
	}

	_, err = decoder.Get(count + 1)
	if nil != err {
		fmt.Println(err)
	}
}
