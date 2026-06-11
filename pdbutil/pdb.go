package pdbutil

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/puzpuzpuz/xsync/v3"
	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var NullValue = sql.NullString{String: "", Valid: false}

var EmptyPDB = &protodb.PDBField{}
var EmptyPDBM = &protodb.PDBMsg{}

type pdbCacheItem struct {
	pdb   *protodb.PDBField
	found bool
}

type pdbmCacheItem struct {
	pdbm  *protodb.PDBMsg
	found bool
}

type descriptorCacheKey struct {
	desc protoreflect.Descriptor
}

type msgFieldInfo struct {
	field protoreflect.FieldDescriptor
	name  string
	lower string
}

var pdbCache = xsync.NewMapOf[descriptorCacheKey, pdbCacheItem]()
var pdbmCache = xsync.NewMapOf[descriptorCacheKey, pdbmCacheItem]()
var msgFieldInfoCache = xsync.NewMapOf[descriptorCacheKey, []msgFieldInfo]()

func GetPDB(fieldDescriptor protoreflect.FieldDescriptor) (pdb *protodb.PDBField, found bool) {
	cacheKey := makeDescriptorCacheKey(fieldDescriptor)
	if cached, ok := pdbCache.Load(cacheKey); ok {
		return cached.pdb, cached.found
	}

	fieldOptions := fieldDescriptor.Options()
	if fieldOptions == nil {
		pdbCache.Store(cacheKey, pdbCacheItem{pdb: EmptyPDB, found: false})
		return EmptyPDB, false
	}

	if !proto.HasExtension(fieldOptions, protodb.E_Pdb) {
		pdbCache.Store(cacheKey, pdbCacheItem{pdb: EmptyPDB, found: false})
		return EmptyPDB, false
	}

	pdb, found = proto.GetExtension(fieldOptions, protodb.E_Pdb).(*protodb.PDBField)
	if !found {
		pdb = EmptyPDB
	}
	pdbCache.Store(cacheKey, pdbCacheItem{pdb: pdb, found: found})
	return pdb, found
}

func GetPDBM(msgDescriptor protoreflect.MessageDescriptor) (pdbm *protodb.PDBMsg, found bool) {
	cacheKey := makeDescriptorCacheKey(msgDescriptor)
	if cached, ok := pdbmCache.Load(cacheKey); ok {
		return cached.pdbm, cached.found
	}

	msgOptions := msgDescriptor.Options()
	if msgOptions == nil {
		pdbmCache.Store(cacheKey, pdbmCacheItem{pdbm: EmptyPDBM, found: false})
		return EmptyPDBM, false
	}

	if !proto.HasExtension(msgOptions, protodb.E_Pdbm) {
		pdbmCache.Store(cacheKey, pdbmCacheItem{pdbm: EmptyPDBM, found: false})
		return EmptyPDBM, false
	}

	pdbm, found = proto.GetExtension(msgOptions, protodb.E_Pdbm).(*protodb.PDBMsg)
	if !found {
		pdbm = EmptyPDBM
	}
	pdbmCache.Store(cacheKey, pdbmCacheItem{pdbm: pdbm, found: found})
	return pdbm, found
}

func makeDescriptorCacheKey(desc protoreflect.Descriptor) descriptorCacheKey {
	return descriptorCacheKey{desc: desc}
}

func getMsgFieldInfos(msgFieldsDesc protoreflect.FieldDescriptors) []msgFieldInfo {
	if msgFieldsDesc.Len() == 0 {
		return nil
	}

	cacheKey := makeDescriptorCacheKey(msgFieldsDesc.Get(0).Parent())
	if infos, ok := msgFieldInfoCache.Load(cacheKey); ok {
		return infos
	}

	infos := make([]msgFieldInfo, msgFieldsDesc.Len())
	for i := 0; i < msgFieldsDesc.Len(); i++ {
		fieldDesc := msgFieldsDesc.Get(i)
		name := string(fieldDesc.Name())
		infos[i] = msgFieldInfo{
			field: fieldDesc,
			name:  name,
			lower: strings.ToLower(name),
		}
	}
	if cached, loaded := msgFieldInfoCache.LoadOrStore(cacheKey, infos); loaded {
		return cached
	}
	return infos
}

func IsZeroValue(val interface{}) bool {
	switch v := val.(type) {
	case nil:
		return false
	case string:
		return len(v) == 0 || v == "0"
	case int:
		return v == 0
	case int8:
		return v == 0
	case int16:
		return v == 0
	case int32:
		return v == 0
	case int64:
		return v == 0
	case uint:
		return v == 0
	case uint8:
		return v == 0
	case uint16:
		return v == 0
	case uint32:
		return v == 0
	case uint64:
		return v == 0
	case float32:
		return v == 0
	case float64:
		return v == 0
	case bool:
		return false
	}
	valStr := fmt.Sprint(val)
	return len(valStr) == 0 || valStr == "0"
}

func MaybeNull(val interface{}, field protoreflect.FieldDescriptor, fieldpdb *protodb.PDBField) interface{} {
	if IsZeroValue(val) {
		return NullValue
	}
	return val
}

// BuildMsgFieldsMap build msgFieldsMap, if columnNames is nil, return all msg fields
func BuildMsgFieldsMap(fieldNames []string, msgFieldsDesc protoreflect.FieldDescriptors, nameLowercase bool) map[string]protoreflect.FieldDescriptor {
	var columnNamesMap map[string]struct{}
	if fieldNames != nil {
		columnNamesMap = make(map[string]struct{}, len(fieldNames))
		for _, columnName := range fieldNames {
			columnNamesMap[strings.ToLower(columnName)] = struct{}{}
		}
	}

	resultCap := msgFieldsDesc.Len()
	if fieldNames != nil && len(fieldNames) < resultCap {
		resultCap = len(fieldNames)
	}
	msgFieldsMap := make(map[string]protoreflect.FieldDescriptor, resultCap)

	for _, fieldInfo := range getMsgFieldInfos(msgFieldsDesc) {
		if fieldNames == nil {
			if nameLowercase {
				msgFieldsMap[fieldInfo.lower] = fieldInfo.field
			} else {
				msgFieldsMap[fieldInfo.name] = fieldInfo.field
			}
			continue
		}
		if _, ok := columnNamesMap[fieldInfo.lower]; ok {
			if nameLowercase {
				msgFieldsMap[fieldInfo.lower] = fieldInfo.field
			} else {
				msgFieldsMap[fieldInfo.name] = fieldInfo.field
			}
		}
	}

	return msgFieldsMap
}

// GetPrimaryKeyFieldDescs get primary key field descriptors, primaryKey(lowercase) -> field descriptor
func GetPrimaryKeyFieldDescs(msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors, nameLowercase bool) map[string]protoreflect.FieldDescriptor {
	result := make(map[string]protoreflect.FieldDescriptor)

	if nameLowercase {
		for _, fieldInfo := range getMsgFieldInfos(msgFieldDescs) {
			field := fieldInfo.field
			fieldPdb, _ := GetPDB(field)
			if fieldPdb != nil && fieldPdb.IsPrimary() {
				result[fieldInfo.lower] = field
			}
		}
		return result
	}

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)
		fieldPdb, _ := GetPDB(field)
		if fieldPdb != nil && fieldPdb.IsPrimary() {
			result[string(field.Name())] = field
		}
	}

	return result
}

type TuniqueConstraints struct {
	//if is primary, = primary
	//if is unique and not specify unique name, = field name
	//if is unique and specify unique name, = unique name
	PrimaryOrUniqueName string
	Fields              map[string]protoreflect.FieldDescriptor
}

// GetPrimaryKeyOrUniqueFieldDescs get primary key or unique field descriptors, constraint name -> *TuniqueConstraints
func GetPrimaryKeyOrUniqueFieldDescs(msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors, nameLowercase bool) map[string]*TuniqueConstraints {
	r := make(map[string]*TuniqueConstraints)

	if nameLowercase {
		for _, fieldInfo := range getMsgFieldInfos(msgFieldDescs) {
			field := fieldInfo.field
			fieldPdb, _ := GetPDB(field)
			if fieldPdb != nil && (fieldPdb.IsPrimary() || fieldPdb.IsUnique() || len(fieldPdb.UniqueName) > 0) {
				fieldName := fieldInfo.lower
				uniqueName := fieldPdb.UniqueName
				if fieldPdb.IsPrimary() {
					uniqueName = "primary"
				}
				if len(uniqueName) == 0 {
					uniqueName = fieldName
				}

				item, ok := r[uniqueName]
				if !ok {
					item = &TuniqueConstraints{
						PrimaryOrUniqueName: uniqueName,
						Fields:              make(map[string]protoreflect.FieldDescriptor),
					}
					r[uniqueName] = item
				}
				item.Fields[fieldName] = field
			}
		}
		return r
	}

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)
		fieldPdb, _ := GetPDB(field)
		if fieldPdb != nil && (fieldPdb.IsPrimary() || fieldPdb.IsUnique() || len(fieldPdb.UniqueName) > 0) {
			fieldName := string(field.Name())

			uniqueName := fieldPdb.UniqueName
			if fieldPdb.IsPrimary() {
				uniqueName = "primary"
			}
			if len(uniqueName) == 0 {
				uniqueName = fieldName
			}

			item, ok := r[uniqueName]
			if !ok {
				item = &TuniqueConstraints{
					PrimaryOrUniqueName: uniqueName,
					Fields:              make(map[string]protoreflect.FieldDescriptor),
				}
				r[uniqueName] = item
			}
			item.Fields[fieldName] = field

		}
	}

	return r
}
