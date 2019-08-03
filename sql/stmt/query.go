package stmt

import (
	"github.com/lindb/lindb/pkg/interval"
	"github.com/lindb/lindb/pkg/timeutil"
)

// Query represents search statement
type Query struct {
	MetricName  string // like table name
	SelectItems []Expr // select list, such as field, function call, math expression etc.
	Condition   Expr   // tag filter condition expression

	TimeRange    timeutil.TimeRange // query time range
	Interval     int64              // down sampling interval
	IntervalType interval.Type      // interval type calc based on down sampling interval

	GroupBy []string // group by
	Limit   int      // num. of time series list for result
}
