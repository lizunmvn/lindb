package memdb

import (
	"fmt"
	"testing"

	"github.com/lindb/lindb/pkg/encoding"
	"github.com/lindb/lindb/pkg/field"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestSimpleSegmentStore(t *testing.T) {
	aggFunc := field.GetAggFunc(field.Sum)
	store := newSimpleFieldStore(0, aggFunc)

	assert.NotNil(t, store)
	ss, ok := store.(*simpleFieldStore)
	assert.True(t, ok)
	assert.Equal(t, aggFunc, ss.AggFunc())

	_, _, err := ss.slotRange()
	assert.NotNil(t, err)

	compress, startSlot, endSlot, err := store.bytes()
	assert.Nil(t, compress)
	assert.NotNil(t, err)
	assert.Equal(t, 0, startSlot)
	assert.Equal(t, 0, endSlot)

	writeCtx := writeContext{
		blockStore:   newBlockStore(30),
		timeInterval: 10,
		metricID:     1,
		familyTime:   0,
	}

	writeCtx.slotIndex = 10
	ss.writeInt(100, writeCtx)
	// memory auto rollup
	writeCtx.slotIndex = 11
	ss.writeInt(110, writeCtx)
	// memory auto rollup
	writeCtx.slotIndex = 10
	ss.writeInt(100, writeCtx)
	// compact because slot out of current time window
	writeCtx.slotIndex = 40
	ss.writeInt(20, writeCtx)
	// compact before time window
	writeCtx.slotIndex = 10
	ss.writeInt(100, writeCtx)
	// compact because slot out of current time window
	writeCtx.slotIndex = 41
	ss.writeInt(50, writeCtx)

	compress, startSlot, endSlot, err = store.bytes()
	assert.Nil(t, err)
	assert.Equal(t, 10, startSlot)
	assert.Equal(t, 41, endSlot)

	startSlot, endSlot, err = store.slotRange()
	assert.Nil(t, err)
	assert.Equal(t, 10, startSlot)
	assert.Equal(t, 41, endSlot)

	tsd := encoding.NewTSDDecoder(compress)
	assert.Equal(t, 10, tsd.StartTime())
	assert.Equal(t, 41, tsd.EndTime())
	assert.True(t, tsd.HasValueWithSlot(0))
	assert.Equal(t, int64(300), encoding.ZigZagDecode(tsd.Value()))
	assert.True(t, tsd.HasValueWithSlot(1))
	assert.Equal(t, int64(110), encoding.ZigZagDecode(tsd.Value()))
	for i := 1; i < 30; i++ {
		assert.False(t, tsd.HasValueWithSlot(i))
	}
	assert.True(t, tsd.HasValueWithSlot(30))
	assert.Equal(t, int64(20), encoding.ZigZagDecode(tsd.Value()))

	assert.True(t, tsd.HasValueWithSlot(31))
	assert.Equal(t, int64(50), encoding.ZigZagDecode(tsd.Value()))

	// write float test
	writeCtx.slotIndex = 10
	ss.writeFloat(10, writeCtx)
}

func Test_sStore_error(t *testing.T) {
	store := newSimpleFieldStore(0, field.GetAggFunc(field.Sum))
	ss, _ := store.(*simpleFieldStore)
	// compact error test
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockBlock := NewMockblock(ctrl)
	mockBlock.EXPECT().compact(gomock.Any()).Return(0, 0, fmt.Errorf("compat error")).AnyTimes()
	mockBlock.EXPECT().setIntValue(gomock.Any(), gomock.Any()).Return().AnyTimes()
	mockBlock.EXPECT().getStartTime().Return(12).AnyTimes()
	mockBlock.EXPECT().getEndTime().Return(40).AnyTimes()
	ss.block = mockBlock
	_, _, _, err := ss.bytes()
	assert.NotNil(t, err)

	writeCtx := writeContext{
		blockStore:   newBlockStore(30),
		timeInterval: 10,
		metricID:     1,
		familyTime:   0,
	}

	writeCtx.slotIndex = 10
	ss.writeInt(100, writeCtx)
	// memory auto rollup
	writeCtx.slotIndex = 11
	ss.writeInt(110, writeCtx)
}

func BenchmarkSimpleSegmentStore(b *testing.B) {
	aggFunc := field.GetAggFunc(field.Sum)
	store := newSimpleFieldStore(0, aggFunc)
	ss, _ := store.(*simpleFieldStore)

	writeCtx := writeContext{
		blockStore:   newBlockStore(30),
		timeInterval: 10,
		metricID:     1,
		familyTime:   0,
	}

	writeCtx.slotIndex = 10
	ss.writeInt(100, writeCtx)
	// memory auto rollup
	writeCtx.slotIndex = 11
	ss.writeInt(110, writeCtx)
	// memory auto rollup
	writeCtx.slotIndex = 10
	ss.writeInt(100, writeCtx)
	// compact because slot out of current time window
	writeCtx.slotIndex = 40
	ss.writeInt(20, writeCtx)
	// compact before time window
	writeCtx.slotIndex = 10
	ss.writeInt(100, writeCtx)
	// compact because slot out of current time window
	writeCtx.slotIndex = 41
	ss.writeInt(50, writeCtx)

	store.bytes()
}
