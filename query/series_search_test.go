package query

import (
	"errors"
	"testing"

	"github.com/RoaringBitmap/roaring"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/lindb/lindb/sql"
	"github.com/lindb/lindb/sql/stmt"
	"github.com/lindb/lindb/tsdb/index"
	"github.com/lindb/lindb/tsdb/series"
)

func TestSampleCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockFilter := index.NewMockSeriesIDsFilter(ctrl)
	s := mockSeriesIDSet(int64(1), roaring.BitmapOf(1, 2, 3, 4))

	query, _ := sql.Parse("select f from cpu")
	search := newSeriesSearch(1, mockFilter, query)
	resultSet, _ := search.Search()
	assert.Nil(t, resultSet)

	query, _ = sql.Parse("select f from cpu where ip='1.1.1.1'")
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "ip", Value: "1.1.1.1"}, query.TimeRange).
		Return(s, nil)
	search = newSeriesSearch(1, mockFilter, query)
	resultSet, _ = search.Search()
	assert.Equal(t, *s, *resultSet)

	query, _ = sql.Parse("select f from cpu where ip like '1.1.*.1'")
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.LikeExpr{Key: "ip", Value: "1.1.*.1"}, query.TimeRange).
		Return(s, nil)
	search = newSeriesSearch(1, mockFilter, query)
	resultSet, _ = search.Search()
	assert.Equal(t, *s, *resultSet)

	query, _ = sql.Parse("select f from cpu where ip =~ '1.1.*.1'")
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.RegexExpr{Key: "ip", Regexp: "1.1.*.1"}, query.TimeRange).
		Return(s, nil)
	search = newSeriesSearch(1, mockFilter, query)
	resultSet, _ = search.Search()
	assert.Equal(t, *s, *resultSet)

	query, _ = sql.Parse("select f from cpu where ip in ('1.1.1.1','1.1.3.3')")
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.InExpr{Key: "ip", Values: []string{"1.1.1.1", "1.1.3.3"}}, query.TimeRange).
		Return(s, nil)
	search = newSeriesSearch(1, mockFilter, query)
	resultSet, _ = search.Search()
	assert.Equal(t, *s, *resultSet)

	// search error
	query, _ = sql.Parse("select f from cpu where ip='1.1.1.1'")
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "ip", Value: "1.1.1.1"}, query.TimeRange).
		Return(nil, errors.New("search error"))
	search = newSeriesSearch(1, mockFilter, query)
	resultSet, err := search.Search()
	assert.Nil(t, resultSet)
	assert.NotNil(t, err)
}

func TestNotCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockFilter := index.NewMockSeriesIDsFilter(ctrl)

	query, _ := sql.Parse("select f from cpu where ip!='1.1.1.1'")
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "ip", Value: "1.1.1.1"}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(3, 4)), nil)

	mockFilter.EXPECT().
		GetSeriesIDsForTag(uint32(1), "ip", query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(1, 2, 3, 4)), nil)
	search := newSeriesSearch(1, mockFilter, query)
	resultSet, _ := search.Search()
	assert.Equal(t, *mockSeriesIDSet(int64(11), roaring.BitmapOf(1, 2)), *resultSet)

	// error
	query, _ = sql.Parse("select f from cpu where ip!='1.1.1.1'")
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "ip", Value: "1.1.1.1"}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(3, 4)), nil)

	mockFilter.EXPECT().
		GetSeriesIDsForTag(uint32(1), "ip", query.TimeRange).
		Return(nil, errors.New("get series ids error"))
	search = newSeriesSearch(1, mockFilter, query)
	resultSet, err := search.Search()
	assert.Nil(t, resultSet)
	assert.NotNil(t, err)
}

func TestBinaryCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockFilter := index.NewMockSeriesIDsFilter(ctrl)

	// and
	query, _ := sql.Parse("select f from cpu " +
		"where ip='1.1.1.1' and path='/data' and time>'20190410 00:00:00' and time<'20190410 10:00:00'")
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "ip", Value: "1.1.1.1"}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(1, 2, 3, 4)), nil)
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "path", Value: "/data"}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(3, 5)), nil)
	search := newSeriesSearch(1, mockFilter, query)
	resultSet, _ := search.Search()
	assert.Equal(t, *mockSeriesIDSet(int64(11), roaring.BitmapOf(3)), *resultSet)

	// or
	mockFilter2 := index.NewMockSeriesIDsFilter(ctrl)
	query, _ = sql.Parse("select f from cpu " +
		"where ip='1.1.1.1' or path='/data' and time>'20190410 00:00:00' and time<'20190410 10:00:00'")
	mockFilter2.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "ip", Value: "1.1.1.1"}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(1, 2, 3, 4)), nil)
	mockFilter2.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "path", Value: "/data"}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(3, 5)), nil)
	search = newSeriesSearch(1, mockFilter2, query)
	resultSet, _ = search.Search()
	assert.Equal(t, *mockSeriesIDSet(int64(11), roaring.BitmapOf(1, 2, 3, 4, 5)), *resultSet)

	// error
	mockFilter3 := index.NewMockSeriesIDsFilter(ctrl)
	mockFilter3.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "ip", Value: "1.1.1.1"}, query.TimeRange).
		Return(nil, errors.New("left error"))
	search = newSeriesSearch(1, mockFilter3, query)
	resultSet, err := search.Search()
	assert.Nil(t, resultSet)
	assert.NotNil(t, err)

	mockFilter4 := index.NewMockSeriesIDsFilter(ctrl)
	query, _ = sql.Parse("select f from cpu " +
		"where ip='1.1.1.1' or path='/data' and time>'20190410 00:00:00' and time<'20190410 10:00:00'")
	mockFilter4.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "ip", Value: "1.1.1.1"}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(1, 2, 3, 4)), nil)
	mockFilter4.EXPECT().
		FindSeriesIDsByExpr(uint32(1), &stmt.EqualsExpr{Key: "path", Value: "/data"}, query.TimeRange).
		Return(nil, errors.New("right error"))
	search = newSeriesSearch(1, mockFilter4, query)
	resultSet, err = search.Search()
	assert.Nil(t, resultSet)
	assert.NotNil(t, err)
}

func TestComplexCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockFilter := index.NewMockSeriesIDsFilter(ctrl)

	query, _ := sql.Parse("select f from cpu" +
		" where (ip not in ('1.1.1.1','2.2.2.2') and region='sh') and (path='/data' or path='/home')")
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(10), &stmt.InExpr{Key: "ip", Values: []string{"1.1.1.1", "2.2.2.2"}}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(1, 2, 4)), nil)
	mockFilter.EXPECT().
		GetSeriesIDsForTag(uint32(10), "ip", query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(1, 2, 3, 4, 6, 7, 8)), nil)
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(10), &stmt.EqualsExpr{Key: "region", Value: "sh"}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(2, 3, 4, 7)), nil)
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(10), &stmt.EqualsExpr{Key: "path", Value: "/data"}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(3, 5)), nil)
	mockFilter.EXPECT().
		FindSeriesIDsByExpr(uint32(10), &stmt.EqualsExpr{Key: "path", Value: "/home"}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(1)), nil)
	search := newSeriesSearch(10, mockFilter, query)
	resultSet, _ := search.Search()
	// ip not in ('1.1.1.1','2.2.2.2') => 3,6,7,8
	// ip not in ('1.1.1.1','2.2.2.2') and region='sh' => 3,7
	// path='/data' or path='/home' => 1,3,5
	// final => 3
	assert.Equal(t, *mockSeriesIDSet(int64(11), roaring.BitmapOf(3)), *resultSet)

	// error
	mockFilter1 := index.NewMockSeriesIDsFilter(ctrl)
	mockFilter1.EXPECT().
		FindSeriesIDsByExpr(uint32(10), &stmt.InExpr{Key: "ip", Values: []string{"1.1.1.1", "2.2.2.2"}}, query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(1, 2, 4)), nil)
	mockFilter1.EXPECT().
		GetSeriesIDsForTag(uint32(10), "ip", query.TimeRange).
		Return(mockSeriesIDSet(int64(11), roaring.BitmapOf(1, 2, 3, 4, 6, 7, 8)), nil)
	mockFilter1.EXPECT().
		FindSeriesIDsByExpr(uint32(10), &stmt.EqualsExpr{Key: "region", Value: "sh"}, query.TimeRange).
		Return(nil, errors.New("complex error"))
	search = newSeriesSearch(10, mockFilter1, query)
	resultSet, err := search.Search()
	assert.Nil(t, resultSet)
	assert.NotNil(t, err)
}

func mockSeriesIDSet(version int64, ids *roaring.Bitmap) *series.MultiVerSeriesIDSet {
	s := series.NewMultiVerSeriesIDSet()
	s.Add(version, ids)
	return s
}
