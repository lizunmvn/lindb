package memdb

import (
	"fmt"
	"sort"
	"testing"

	"github.com/lindb/lindb/pkg/field"
	pb "github.com/lindb/lindb/rpc/proto/field"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func getMockSStore(ctrl *gomock.Controller, familyTime int64) *MocksStoreINTF {
	mockSStore := NewMocksStoreINTF(ctrl)
	mockSStore.EXPECT().getFamilyTime().Return(familyTime).AnyTimes()
	return mockSStore
}

func Test_newFieldStore(t *testing.T) {
	fStore := newFieldStore("sum", 10, field.SumField)
	assert.NotNil(t, fStore)
	assert.Equal(t, fStore.getFieldType(), field.SumField)
	timeRange, ok := fStore.timeRange(10)
	assert.False(t, ok)
	assert.Equal(t, int64(0), timeRange.Start)
	assert.Equal(t, int64(0), timeRange.End)
}

func Test_fStore_write(t *testing.T) {
	fStore := newFieldStore("sum", 10, field.SumField)
	theFieldStore := fStore.(*fieldStore)
	writeCtx := writeContext{familyTime: 15, blockStore: newBlockStore(30)}

	// unknown field
	theFieldStore.write(&pb.Field{Name: "unknown"}, writeCtx)
	// sum field
	theFieldStore.write(&pb.Field{Name: "sum", Field: &pb.Field_Sum{}}, writeCtx)
	theFieldStore.write(&pb.Field{Name: "sum", Field: &pb.Field_Sum{}}, writeCtx)
}

func Test_fStore_timeRange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fStore := newFieldStore("sum", 10, field.SumField)
	theFieldStore := fStore.(*fieldStore)

	mockSStore1 := getMockSStore(ctrl, 1564300800000)
	mockSStore1.EXPECT().slotRange().Return(1, 10, nil).AnyTimes()
	mockSStore2 := getMockSStore(ctrl, 1564304400000)
	mockSStore2.EXPECT().slotRange().Return(3, 5, nil).AnyTimes()
	mockSStore3 := getMockSStore(ctrl, 1564297200000)
	mockSStore3.EXPECT().slotRange().Return(6, 13, fmt.Errorf("error")).AnyTimes()
	mockSStore4 := getMockSStore(ctrl, 1564308000000)
	mockSStore4.EXPECT().slotRange().Return(4, 14, nil).AnyTimes()

	// error case

	theFieldStore.insertSStore(mockSStore3)
	timeRange, ok := theFieldStore.timeRange(10 * 1000)
	assert.Equal(t, int64(0), timeRange.Start)
	assert.Equal(t, int64(0), timeRange.End)
	assert.False(t, ok)

	theFieldStore.insertSStore(mockSStore1)
	timeRange, ok = theFieldStore.timeRange(10 * 1000)
	assert.Equal(t, int64(1564300810000), timeRange.Start)
	assert.Equal(t, int64(1564300900000), timeRange.End)
	assert.True(t, ok)

	theFieldStore.insertSStore(mockSStore2)
	theFieldStore.insertSStore(mockSStore4)
	timeRange, ok = theFieldStore.timeRange(10 * 1000)
	assert.Equal(t, int64(1564300810000), timeRange.Start)
	assert.Equal(t, int64(1564308140000), timeRange.End)
	assert.True(t, ok)
}

func Test_fStore_flushFieldTo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fStore := newFieldStore("sum", 10, field.SumField)
	theFieldStore := fStore.(*fieldStore)

	mockTF := makeMockTableFlusher(ctrl)
	mockSStore1 := getMockSStore(ctrl, 1564304400000)
	mockSStore1.EXPECT().bytes().Return(nil, 0, 0, fmt.Errorf("error")).AnyTimes()
	mockSStore2 := getMockSStore(ctrl, 1564308000000)
	mockSStore2.EXPECT().bytes().Return(nil, 1, 212, nil).AnyTimes()

	theFieldStore.insertSStore(mockSStore1)
	theFieldStore.insertSStore(mockSStore2)

	assert.Len(t, theFieldStore.sStoreNodes, 2)
	// familyTime not exist
	assert.False(t, theFieldStore.flushFieldTo(mockTF, 1564297200000))
	assert.Len(t, theFieldStore.sStoreNodes, 2)
	// mock error
	assert.False(t, theFieldStore.flushFieldTo(mockTF, 1564304400000))
	assert.Len(t, theFieldStore.sStoreNodes, 1)
	// mock ok
	assert.True(t, theFieldStore.flushFieldTo(mockTF, 1564308000000))
	assert.Len(t, theFieldStore.sStoreNodes, 0)
}

func Test_fStore_removeSStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fsINTF := newFieldStore("sum", 1, field.SumField)
	fs := fsINTF.(*fieldStore)
	// segments empty
	fs.removeSStore(0)
	fs.removeSStore(1)

	// assert sorted
	fs.insertSStore(getMockSStore(ctrl, 1))
	fs.insertSStore(getMockSStore(ctrl, 9))
	fs.insertSStore(getMockSStore(ctrl, 2))
	fs.insertSStore(getMockSStore(ctrl, 3))
	fs.insertSStore(getMockSStore(ctrl, 7))
	fs.insertSStore(getMockSStore(ctrl, 5))
	assert.True(t, sort.IsSorted(fs.sStoreNodes))
	// remove greater
	fs.removeSStore(10)
	// remove not exist
	fs.removeSStore(8)
	// remove smaller
	fs.removeSStore(0)
	// remove existed
	fs.removeSStore(9)
	fs.removeSStore(1)
	fs.removeSStore(3)
	fs.removeSStore(4)
	fs.removeSStore(2)
	fs.removeSStore(7)

}
