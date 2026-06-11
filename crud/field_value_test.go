package crud

import (
	"reflect"
	"testing"

	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestGetSQLFieldValueScalarFastPath(t *testing.T) {
	msg := &protodb.TableQueryReq{
		SchemeName:      "public",
		TableName:       "User",
		Limit:           10,
		Offset:          20,
		PreferBatchSize: 30,
		MsgFormat:       1,
	}
	fields := msg.ProtoReflect().Descriptor().Fields()

	tests := []struct {
		name  string
		field string
		want  any
	}{
		{name: "string", field: "SchemeName", want: "public"},
		{name: "int32", field: "Limit", want: int32(10)},
		{name: "int64", field: "Offset", want: int64(20)},
		{name: "second_int32", field: "PreferBatchSize", want: int32(30)},
		{name: "msg_format", field: "MsgFormat", want: int32(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getSQLFieldValue(msg, fields.ByName(protoreflect.Name(tt.field)))
			if err != nil {
				t.Fatalf("getSQLFieldValue: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("getSQLFieldValue(%s) = %#v (%T), want %#v (%T)", tt.field, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestGetSQLFieldValueComplexFallback(t *testing.T) {
	msg := &protodb.TableQueryReq{
		ResultColumnNames: []string{"id", "name"},
		Where:             map[string]string{"TableName": "User"},
	}
	fields := msg.ProtoReflect().Descriptor().Fields()

	gotList, err := getSQLFieldValue(msg, fields.ByName(protoreflect.Name("ResultColumnNames")))
	if err != nil {
		t.Fatalf("getSQLFieldValue list: %v", err)
	}
	if !reflect.DeepEqual(gotList, []string{"id", "name"}) {
		t.Fatalf("list fallback = %#v", gotList)
	}

	gotMap, err := getSQLFieldValue(msg, fields.ByName(protoreflect.Name("Where")))
	if err != nil {
		t.Fatalf("getSQLFieldValue map: %v", err)
	}
	if !reflect.DeepEqual(gotMap, map[string]string{"TableName": "User"}) {
		t.Fatalf("map fallback = %#v", gotMap)
	}
}
