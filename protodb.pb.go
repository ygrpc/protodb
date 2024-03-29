// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v4.25.3
// source: protodb.proto

package protodb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type PDBFile struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// name style(msg & field)
	// empty='go': default, go name style, better performance in crud operation in
	// go (like: UserName) 'snake': snake name style (like: user_name)
	NameStyle string `protobuf:"bytes,6,opt,name=NameStyle,proto3" json:"NameStyle,omitempty"`
}

func (x *PDBFile) Reset() {
	*x = PDBFile{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protodb_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PDBFile) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PDBFile) ProtoMessage() {}

func (x *PDBFile) ProtoReflect() protoreflect.Message {
	mi := &file_protodb_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PDBFile.ProtoReflect.Descriptor instead.
func (*PDBFile) Descriptor() ([]byte, []int) {
	return file_protodb_proto_rawDescGZIP(), []int{0}
}

func (x *PDBFile) GetNameStyle() string {
	if x != nil {
		return x.NameStyle
	}
	return ""
}

type PDBMsg struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// when table primary key include more than one field,need specify the primary
	// key in pdbmsg
	PrimaryKeys []string `protobuf:"bytes,1,rep,name=PrimaryKeys,proto3" json:"PrimaryKeys,omitempty"`
	// sql prepends before create table
	SQLPrepends []string `protobuf:"bytes,2,rep,name=SQLPrepends,proto3" json:"SQLPrepends,omitempty"`
	// sql appends before )
	SQLAppends []string `protobuf:"bytes,3,rep,name=SQLAppends,proto3" json:"SQLAppends,omitempty"`
	// sql appends after ) before ;
	SQLAppendsAfter []string `protobuf:"bytes,4,rep,name=SQLAppendsAfter,proto3" json:"SQLAppendsAfter,omitempty"`
	// sql appends after ;
	SQLAppendsEnd []string `protobuf:"bytes,5,rep,name=SQLAppendsEnd,proto3" json:"SQLAppendsEnd,omitempty"`
	// generate proto msg {{msg}}List in  xxx.list.proto
	// 0: auto if msg name start with db then generate {{msg}}List
	// 1: always generate {{msg}}List
	// 4: never generate {{msg}}List
	MsgList int32 `protobuf:"varint,6,opt,name=MsgList,proto3" json:"MsgList,omitempty"`
}

func (x *PDBMsg) Reset() {
	*x = PDBMsg{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protodb_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PDBMsg) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PDBMsg) ProtoMessage() {}

func (x *PDBMsg) ProtoReflect() protoreflect.Message {
	mi := &file_protodb_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PDBMsg.ProtoReflect.Descriptor instead.
func (*PDBMsg) Descriptor() ([]byte, []int) {
	return file_protodb_proto_rawDescGZIP(), []int{1}
}

func (x *PDBMsg) GetPrimaryKeys() []string {
	if x != nil {
		return x.PrimaryKeys
	}
	return nil
}

func (x *PDBMsg) GetSQLPrepends() []string {
	if x != nil {
		return x.SQLPrepends
	}
	return nil
}

func (x *PDBMsg) GetSQLAppends() []string {
	if x != nil {
		return x.SQLAppends
	}
	return nil
}

func (x *PDBMsg) GetSQLAppendsAfter() []string {
	if x != nil {
		return x.SQLAppendsAfter
	}
	return nil
}

func (x *PDBMsg) GetSQLAppendsEnd() []string {
	if x != nil {
		return x.SQLAppendsEnd
	}
	return nil
}

func (x *PDBMsg) GetMsgList() int32 {
	if x != nil {
		return x.MsgList
	}
	return 0
}

type PDBField struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// do not generate db field in create table
	// when in update, do not update this field
	NotDB bool `protobuf:"varint,1,opt,name=NotDB,proto3" json:"NotDB,omitempty"`
	// is primary key
	IsPrimaryKey bool `protobuf:"varint,2,opt,name=IsPrimaryKey,proto3" json:"IsPrimaryKey,omitempty"`
	// is unique key
	IsUniqueKey bool `protobuf:"varint,3,opt,name=IsUniqueKey,proto3" json:"IsUniqueKey,omitempty"`
	// is not null
	IsNotNull bool `protobuf:"varint,4,opt,name=IsNotNull,proto3" json:"IsNotNull,omitempty"`
	// reference to other table, sql like:  REFERENCES other_table(other_field)
	Reference string `protobuf:"bytes,5,opt,name=Reference,proto3" json:"Reference,omitempty"`
	// default value
	DefaultValue string `protobuf:"bytes,6,opt,name=DefaultValue,proto3" json:"DefaultValue,omitempty"`
	// append sql before ,
	SQLAppends []string `protobuf:"bytes,7,rep,name=SQLAppends,proto3" json:"SQLAppends,omitempty"`
	// append sql after ,
	SQLAppendsEnd []string `protobuf:"bytes,8,rep,name=SQLAppendsEnd,proto3" json:"SQLAppendsEnd,omitempty"`
	// db no update
	// when in update, do not update this field, for example, create time
	NoUpdate bool `protobuf:"varint,9,opt,name=NoUpdate,proto3" json:"NoUpdate,omitempty"`
	// serial type 0:not serial type 1:smallint 4:int 8:bigint
	// strong advice not use serial type,it's hard in distributed system
	SerialType int32 `protobuf:"varint,10,opt,name=SerialType,proto3" json:"SerialType,omitempty"`
}

func (x *PDBField) Reset() {
	*x = PDBField{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protodb_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PDBField) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PDBField) ProtoMessage() {}

func (x *PDBField) ProtoReflect() protoreflect.Message {
	mi := &file_protodb_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PDBField.ProtoReflect.Descriptor instead.
func (*PDBField) Descriptor() ([]byte, []int) {
	return file_protodb_proto_rawDescGZIP(), []int{2}
}

func (x *PDBField) GetNotDB() bool {
	if x != nil {
		return x.NotDB
	}
	return false
}

func (x *PDBField) GetIsPrimaryKey() bool {
	if x != nil {
		return x.IsPrimaryKey
	}
	return false
}

func (x *PDBField) GetIsUniqueKey() bool {
	if x != nil {
		return x.IsUniqueKey
	}
	return false
}

func (x *PDBField) GetIsNotNull() bool {
	if x != nil {
		return x.IsNotNull
	}
	return false
}

func (x *PDBField) GetReference() string {
	if x != nil {
		return x.Reference
	}
	return ""
}

func (x *PDBField) GetDefaultValue() string {
	if x != nil {
		return x.DefaultValue
	}
	return ""
}

func (x *PDBField) GetSQLAppends() []string {
	if x != nil {
		return x.SQLAppends
	}
	return nil
}

func (x *PDBField) GetSQLAppendsEnd() []string {
	if x != nil {
		return x.SQLAppendsEnd
	}
	return nil
}

func (x *PDBField) GetNoUpdate() bool {
	if x != nil {
		return x.NoUpdate
	}
	return false
}

func (x *PDBField) GetSerialType() int32 {
	if x != nil {
		return x.SerialType
	}
	return 0
}

var file_protodb_proto_extTypes = []protoimpl.ExtensionInfo{
	{
		ExtendedType:  (*descriptorpb.FileOptions)(nil),
		ExtensionType: (*PDBFile)(nil),
		Field:         1888,
		Name:          "protodb.pdbf",
		Tag:           "bytes,1888,opt,name=pdbf",
		Filename:      "protodb.proto",
	},
	{
		ExtendedType:  (*descriptorpb.MessageOptions)(nil),
		ExtensionType: (*PDBMsg)(nil),
		Field:         1888,
		Name:          "protodb.pdbm",
		Tag:           "bytes,1888,opt,name=pdbm",
		Filename:      "protodb.proto",
	},
	{
		ExtendedType:  (*descriptorpb.FieldOptions)(nil),
		ExtensionType: (*PDBField)(nil),
		Field:         1888,
		Name:          "protodb.pdb",
		Tag:           "bytes,1888,opt,name=pdb",
		Filename:      "protodb.proto",
	},
}

// Extension fields to descriptorpb.FileOptions.
var (
	// optional protodb.PDBFile pdbf = 1888;
	E_Pdbf = &file_protodb_proto_extTypes[0]
)

// Extension fields to descriptorpb.MessageOptions.
var (
	// optional protodb.PDBMsg pdbm = 1888;
	E_Pdbm = &file_protodb_proto_extTypes[1]
)

// Extension fields to descriptorpb.FieldOptions.
var (
	// optional protodb.PDBField pdb = 1888;
	E_Pdb = &file_protodb_proto_extTypes[2]
)

var File_protodb_proto protoreflect.FileDescriptor

var file_protodb_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64, 0x62, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x07, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64, 0x62, 0x1a, 0x20, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x27, 0x0a, 0x07, 0x50, 0x44,
	0x42, 0x46, 0x69, 0x6c, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x4e, 0x61, 0x6d, 0x65, 0x53, 0x74, 0x79,
	0x6c, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x4e, 0x61, 0x6d, 0x65, 0x53, 0x74,
	0x79, 0x6c, 0x65, 0x22, 0xd6, 0x01, 0x0a, 0x06, 0x50, 0x44, 0x42, 0x4d, 0x73, 0x67, 0x12, 0x20,
	0x0a, 0x0b, 0x50, 0x72, 0x69, 0x6d, 0x61, 0x72, 0x79, 0x4b, 0x65, 0x79, 0x73, 0x18, 0x01, 0x20,
	0x03, 0x28, 0x09, 0x52, 0x0b, 0x50, 0x72, 0x69, 0x6d, 0x61, 0x72, 0x79, 0x4b, 0x65, 0x79, 0x73,
	0x12, 0x20, 0x0a, 0x0b, 0x53, 0x51, 0x4c, 0x50, 0x72, 0x65, 0x70, 0x65, 0x6e, 0x64, 0x73, 0x18,
	0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0b, 0x53, 0x51, 0x4c, 0x50, 0x72, 0x65, 0x70, 0x65, 0x6e,
	0x64, 0x73, 0x12, 0x1e, 0x0a, 0x0a, 0x53, 0x51, 0x4c, 0x41, 0x70, 0x70, 0x65, 0x6e, 0x64, 0x73,
	0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0a, 0x53, 0x51, 0x4c, 0x41, 0x70, 0x70, 0x65, 0x6e,
	0x64, 0x73, 0x12, 0x28, 0x0a, 0x0f, 0x53, 0x51, 0x4c, 0x41, 0x70, 0x70, 0x65, 0x6e, 0x64, 0x73,
	0x41, 0x66, 0x74, 0x65, 0x72, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0f, 0x53, 0x51, 0x4c,
	0x41, 0x70, 0x70, 0x65, 0x6e, 0x64, 0x73, 0x41, 0x66, 0x74, 0x65, 0x72, 0x12, 0x24, 0x0a, 0x0d,
	0x53, 0x51, 0x4c, 0x41, 0x70, 0x70, 0x65, 0x6e, 0x64, 0x73, 0x45, 0x6e, 0x64, 0x18, 0x05, 0x20,
	0x03, 0x28, 0x09, 0x52, 0x0d, 0x53, 0x51, 0x4c, 0x41, 0x70, 0x70, 0x65, 0x6e, 0x64, 0x73, 0x45,
	0x6e, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x4d, 0x73, 0x67, 0x4c, 0x69, 0x73, 0x74, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x05, 0x52, 0x07, 0x4d, 0x73, 0x67, 0x4c, 0x69, 0x73, 0x74, 0x22, 0xc8, 0x02, 0x0a,
	0x08, 0x50, 0x44, 0x42, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x4e, 0x6f, 0x74,
	0x44, 0x42, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x4e, 0x6f, 0x74, 0x44, 0x42, 0x12,
	0x22, 0x0a, 0x0c, 0x49, 0x73, 0x50, 0x72, 0x69, 0x6d, 0x61, 0x72, 0x79, 0x4b, 0x65, 0x79, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0c, 0x49, 0x73, 0x50, 0x72, 0x69, 0x6d, 0x61, 0x72, 0x79,
	0x4b, 0x65, 0x79, 0x12, 0x20, 0x0a, 0x0b, 0x49, 0x73, 0x55, 0x6e, 0x69, 0x71, 0x75, 0x65, 0x4b,
	0x65, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0b, 0x49, 0x73, 0x55, 0x6e, 0x69, 0x71,
	0x75, 0x65, 0x4b, 0x65, 0x79, 0x12, 0x1c, 0x0a, 0x09, 0x49, 0x73, 0x4e, 0x6f, 0x74, 0x4e, 0x75,
	0x6c, 0x6c, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x49, 0x73, 0x4e, 0x6f, 0x74, 0x4e,
	0x75, 0x6c, 0x6c, 0x12, 0x1c, 0x0a, 0x09, 0x52, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e, 0x63, 0x65,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x52, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e, 0x63,
	0x65, 0x12, 0x22, 0x0a, 0x0c, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x56, 0x61, 0x6c, 0x75,
	0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x53, 0x51, 0x4c, 0x41, 0x70, 0x70, 0x65,
	0x6e, 0x64, 0x73, 0x18, 0x07, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0a, 0x53, 0x51, 0x4c, 0x41, 0x70,
	0x70, 0x65, 0x6e, 0x64, 0x73, 0x12, 0x24, 0x0a, 0x0d, 0x53, 0x51, 0x4c, 0x41, 0x70, 0x70, 0x65,
	0x6e, 0x64, 0x73, 0x45, 0x6e, 0x64, 0x18, 0x08, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0d, 0x53, 0x51,
	0x4c, 0x41, 0x70, 0x70, 0x65, 0x6e, 0x64, 0x73, 0x45, 0x6e, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x4e,
	0x6f, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x4e,
	0x6f, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x53, 0x65, 0x72, 0x69, 0x61,
	0x6c, 0x54, 0x79, 0x70, 0x65, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0a, 0x53, 0x65, 0x72,
	0x69, 0x61, 0x6c, 0x54, 0x79, 0x70, 0x65, 0x3a, 0x46, 0x0a, 0x04, 0x70, 0x64, 0x62, 0x66, 0x12,
	0x1c, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x46, 0x69, 0x6c, 0x65, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xe0, 0x0e,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64, 0x62, 0x2e, 0x50,
	0x44, 0x42, 0x46, 0x69, 0x6c, 0x65, 0x52, 0x04, 0x70, 0x64, 0x62, 0x66, 0x88, 0x01, 0x01, 0x3a,
	0x48, 0x0a, 0x04, 0x70, 0x64, 0x62, 0x6d, 0x12, 0x1f, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xe0, 0x0e, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x0f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64, 0x62, 0x2e, 0x50, 0x44, 0x42, 0x4d, 0x73, 0x67,
	0x52, 0x04, 0x70, 0x64, 0x62, 0x6d, 0x88, 0x01, 0x01, 0x3a, 0x46, 0x0a, 0x03, 0x70, 0x64, 0x62,
	0x12, 0x1d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18,
	0xe0, 0x0e, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64, 0x62,
	0x2e, 0x50, 0x44, 0x42, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x52, 0x03, 0x70, 0x64, 0x62, 0x88, 0x01,
	0x01, 0x42, 0x1a, 0x5a, 0x18, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x79, 0x67, 0x72, 0x70, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64, 0x62, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_protodb_proto_rawDescOnce sync.Once
	file_protodb_proto_rawDescData = file_protodb_proto_rawDesc
)

func file_protodb_proto_rawDescGZIP() []byte {
	file_protodb_proto_rawDescOnce.Do(func() {
		file_protodb_proto_rawDescData = protoimpl.X.CompressGZIP(file_protodb_proto_rawDescData)
	})
	return file_protodb_proto_rawDescData
}

var file_protodb_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_protodb_proto_goTypes = []interface{}{
	(*PDBFile)(nil),                     // 0: protodb.PDBFile
	(*PDBMsg)(nil),                      // 1: protodb.PDBMsg
	(*PDBField)(nil),                    // 2: protodb.PDBField
	(*descriptorpb.FileOptions)(nil),    // 3: google.protobuf.FileOptions
	(*descriptorpb.MessageOptions)(nil), // 4: google.protobuf.MessageOptions
	(*descriptorpb.FieldOptions)(nil),   // 5: google.protobuf.FieldOptions
}
var file_protodb_proto_depIdxs = []int32{
	3, // 0: protodb.pdbf:extendee -> google.protobuf.FileOptions
	4, // 1: protodb.pdbm:extendee -> google.protobuf.MessageOptions
	5, // 2: protodb.pdb:extendee -> google.protobuf.FieldOptions
	0, // 3: protodb.pdbf:type_name -> protodb.PDBFile
	1, // 4: protodb.pdbm:type_name -> protodb.PDBMsg
	2, // 5: protodb.pdb:type_name -> protodb.PDBField
	6, // [6:6] is the sub-list for method output_type
	6, // [6:6] is the sub-list for method input_type
	3, // [3:6] is the sub-list for extension type_name
	0, // [0:3] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_protodb_proto_init() }
func file_protodb_proto_init() {
	if File_protodb_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_protodb_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PDBFile); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_protodb_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PDBMsg); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_protodb_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PDBField); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_protodb_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 3,
			NumServices:   0,
		},
		GoTypes:           file_protodb_proto_goTypes,
		DependencyIndexes: file_protodb_proto_depIdxs,
		MessageInfos:      file_protodb_proto_msgTypes,
		ExtensionInfos:    file_protodb_proto_extTypes,
	}.Build()
	File_protodb_proto = out.File
	file_protodb_proto_rawDesc = nil
	file_protodb_proto_goTypes = nil
	file_protodb_proto_depIdxs = nil
}
