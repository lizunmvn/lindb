package memdb

import (
	"github.com/lindb/lindb/pkg/field"
	pb "github.com/lindb/lindb/rpc/proto/field"
)

// getFieldType return field type by given field
func getFieldType(f *pb.Field) field.Type {
	switch f.Field.(type) {
	case *pb.Field_Sum:
		return field.SumField
	case *pb.Field_Min:
		return field.MinField
	default:
		return field.Unknown
	}
}
