package pdbutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/google/uuid"
)

const (
	microsecondsPerSecond = 1000000
	microsecondsPerMinute = 60 * microsecondsPerSecond
	microsecondsPerHour   = 60 * microsecondsPerMinute
)

var WarnInt2StrInSetField = true

// GetField returns the value of the provided obj field. obj can whether
// be a structure or pointer to structure.
func GetField(obj interface{}, name string) (interface{}, error) {
	if !hasValidType(obj, []reflect.Kind{reflect.Struct, reflect.Ptr}) {
		return nil, errors.New("cannot use GetField on a non-struct interface")
	}

	objValue := ReflectValue(obj)
	field := objValue.FieldByName(name)
	if !field.IsValid() {
		return nil, fmt.Errorf("no such field: %s in obj", name)
	}

	return field.Interface(), nil
}

// GetFieldKind returns the kind of the provided obj field. obj can whether
// be a structure or pointer to structure.
func GetFieldKind(obj interface{}, name string) (reflect.Kind, error) {
	if !hasValidType(obj, []reflect.Kind{reflect.Struct, reflect.Ptr}) {
		return reflect.Invalid, errors.New("cannot use GetField on a non-struct interface")
	}

	objValue := ReflectValue(obj)
	field := objValue.FieldByName(name)

	if !field.IsValid() {
		return reflect.Invalid, fmt.Errorf("no such field: %s in obj", name)
	}

	return field.Type().Kind(), nil
}

// GetFieldType returns the kind of the provided obj field. obj can whether
// be a structure or pointer to structure.
func GetFieldType(obj interface{}, name string) (string, error) {
	if !hasValidType(obj, []reflect.Kind{reflect.Struct, reflect.Ptr}) {
		return "", errors.New("cannot use GetField on a non-struct interface")
	}

	objValue := ReflectValue(obj)
	field := objValue.FieldByName(name)

	if !field.IsValid() {
		return "", fmt.Errorf("no such field: %s in obj", name)
	}

	return field.Type().String(), nil
}

// GetFieldTag returns the provided obj field tag value. obj can whether
// be a structure or pointer to structure.
func GetFieldTag(obj interface{}, fieldName, tagKey string) (string, error) {
	if !hasValidType(obj, []reflect.Kind{reflect.Struct, reflect.Ptr}) {
		return "", errors.New("cannot use GetField on a non-struct interface")
	}

	objValue := ReflectValue(obj)
	objType := objValue.Type()

	field, ok := objType.FieldByName(fieldName)
	if !ok {
		return "", fmt.Errorf("no such field: %s in obj", fieldName)
	}

	if !IsExportableField(field) {
		return "", errors.New("cannot GetFieldTag on a non-exported struct field")
	}

	return field.Tag.Get(tagKey), nil
}

// SetField sets the provided obj field with provided value. obj param has
// to be a pointer to a struct, otherwise it will soundly fail. Provided
// value type should match with the struct field you're trying to set.
func SetField(obj interface{}, name string, value interface{}) error {
	val := reflect.ValueOf(value)

	if !val.IsValid() {
		//ignore all invalid val
		return nil
	}

	// Fetch the field reflect.Value
	structValue := reflect.ValueOf(obj).Elem()
	structFieldValue := structValue.FieldByName(name)

	if !structFieldValue.IsValid() {
		return fmt.Errorf("no such field: %s in obj", name)
	}

	// If obj field value is not settable an error is thrown
	if !structFieldValue.CanSet() {
		return fmt.Errorf("cannot set %s field value", name)
	}

	structFieldType := structFieldValue.Type()

	if structFieldType != val.Type() {
		//fmt.Println("name:", name, "v type:", val.Type().String())
		switch structFieldType.Kind() {

		case reflect.String:
			switch val.Type().String() {
			case "time.Time":
				valTime := value.(time.Time)
				val = reflect.ValueOf(TimeISOStr(valTime))
				goto SETVALUE
			case "[]uint8":
				valUuid := value.([]uint8)
				val = reflect.ValueOf(string(valUuid))
				goto SETVALUE

			case "[16]uint8":
				uuid16 := value.([16]uint8)
				uuidv := *(*uuid.UUID)(unsafe.Pointer(&uuid16))
				val = reflect.ValueOf(uuidv.String())
				goto SETVALUE

			case "map[string]interface {}":
				//json
				b, err := json.Marshal(value)
				if err != nil {
					return err
				}
				val = reflect.ValueOf(string(b))
				goto SETVALUE

			case "int32":
				if WarnInt2StrInSetField {
					fmt.Println("setfield to string warn:", name, val.Type().String())
				}
				v32 := value.(int32)
				val = reflect.ValueOf(strconv.Itoa(int(v32)))
				goto SETVALUE
			case "int64":
				usec := value.(int64)

				if strings.Contains(name, "Time") || strings.Contains(name, "time") {
					//time format, Number of microseconds since midnight
					hours := usec / microsecondsPerHour
					usec -= hours * microsecondsPerHour
					minutes := usec / microsecondsPerMinute
					usec -= minutes * microsecondsPerMinute
					seconds := usec / microsecondsPerSecond
					//usec -= seconds * microsecondsPerSecond

					s := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
					val = reflect.ValueOf(s)
				} else {
					s := strconv.FormatInt(usec, 10)
					val = reflect.ValueOf(s)
				}
				goto SETVALUE
			case "*interface {}":
				ptr := value.(*any)
				v, ok := (*ptr).(string)
				if ok {
					val = reflect.ValueOf(v)
					goto SETVALUE
				}
			}
		case reflect.Int32:
			switch val.Type().Kind() {
			case reflect.Uint32:
				valU32 := value.(uint32)
				val = reflect.ValueOf(int32(valU32))
				goto SETVALUE
			case reflect.Int64:
				valI64 := value.(int64)
				val = reflect.ValueOf(int32(valI64))
				goto SETVALUE
			case reflect.Uint64:
				valU64 := value.(uint64)
				val = reflect.ValueOf(int32(valU64))
				goto SETVALUE
			}
		case reflect.Uint32:
			switch val.Type().Kind() {
			case reflect.Int32:
				valI32 := value.(int32)
				val = reflect.ValueOf(uint32(valI32))
				goto SETVALUE
			case reflect.Int64:
				valI64 := value.(int64)
				val = reflect.ValueOf(uint32(valI64))
				goto SETVALUE
			case reflect.Uint64:
				valU64 := value.(uint64)
				val = reflect.ValueOf(uint32(valU64))
				goto SETVALUE
			}
		}
		invalidTypeError := errors.New(name + ": value type didn't match obj field type " + structFieldType.String() + ":" + val.Type().String())
		fmt.Println(name, invalidTypeError)
		return invalidTypeError
	}
SETVALUE:
	structFieldValue.Set(val)
	return nil
}

// HasField checks if the provided field name is part of a struct. obj can whether
// be a structure or pointer to structure.
func HasField(obj interface{}, name string) (bool, error) {
	if !hasValidType(obj, []reflect.Kind{reflect.Struct, reflect.Ptr}) {
		return false, errors.New("cannot use GetField on a non-struct interface")
	}

	objValue := ReflectValue(obj)
	objType := objValue.Type()
	field, ok := objType.FieldByName(name)
	if !ok || !IsExportableField(field) {
		return false, nil
	}

	return true, nil
}

// Fields returns the struct fields names list. obj can whether
// be a structure or pointer to structure.
func Fields(obj interface{}) ([]string, error) {
	return fields(obj, false)
}

// GetStructAllDirectFieldNames 得到一个struct里面所有的导出的字段名,不包含嵌入的匿名字段
func GetStructAllDirectFieldNames(obj interface{}) []string {
	fs, _ := fields(obj, false)
	return fs
}

// GetStructAllFieldNames 得到一个struct里面所有的导出的字段名,包含嵌入的匿名字段
func GetStructAllFieldNames(obj interface{}) []string {
	fs, _ := fields(obj, true)
	return fs
}

// GetStructAllFieldNamesAndJsonTag 得到一个struct里面所有的导出的字段名和对应的json tag名
// fieldnamefirst:是否将字段名作为key,true:fieldname作为key,false:tagname作为key
func GetStructAllFieldNamesAndJsonTag(obj interface{}, deep bool, fieldnamefirst bool) (map[string]string, error) {
	if !hasValidType(obj, []reflect.Kind{reflect.Struct, reflect.Ptr}) {
		return nil, errors.New("cannot use GetField on a non-struct interface")
	}

	objValue := ReflectValue(obj)
	objType := objValue.Type()
	fieldsCount := objType.NumField()

	allfieldAndJsons := make(map[string]string)

	for i := 0; i < fieldsCount; i++ {
		field := objType.Field(i)
		if IsExportableField(field) {
			if deep && field.Anonymous {
				fieldValue := objValue.Field(i)
				subFields, err := GetStructAllFieldNamesAndJsonTag(fieldValue.Interface(), deep, fieldnamefirst)
				if err != nil {
					return nil, fmt.Errorf("cannot get fields in %s: %s", field.Name, err.Error())
				} else {
					if fieldnamefirst {
						for fieldname, jsontag := range subFields {
							allfieldAndJsons[fieldname] = jsontag
						}
					} else {
						for jsontag, fieldname := range subFields {
							allfieldAndJsons[jsontag] = fieldname
						}
					}
				}
			} else {
				jsontag := field.Tag.Get("json")
				before, _, _ := strings.Cut(jsontag, ",")
				if fieldnamefirst {
					allfieldAndJsons[field.Name] = before
				} else {
					allfieldAndJsons[before] = field.Name
				}

			}
		}
	}

	return allfieldAndJsons, nil
}

// FieldsDeep returns "flattened" fields (fields from anonymous
// inner structs are treated as normal fields)
func FieldsDeep(obj interface{}) ([]string, error) {
	return fields(obj, true)
}

func fields(obj interface{}, deep bool) ([]string, error) {
	if !hasValidType(obj, []reflect.Kind{reflect.Struct, reflect.Ptr}) {
		return nil, errors.New("cannot use GetField on a non-struct interface")
	}

	objValue := ReflectValue(obj)
	objType := objValue.Type()
	fieldsCount := objType.NumField()

	var allFields []string
	for i := 0; i < fieldsCount; i++ {
		field := objType.Field(i)
		if IsExportableField(field) {
			if deep && field.Anonymous {
				fieldValue := objValue.Field(i)
				subFields, err := fields(fieldValue.Interface(), deep)
				if err != nil {
					return nil, fmt.Errorf("cannot get fields in %s: %s", field.Name, err.Error())
				}
				allFields = append(allFields, subFields...)
			} else {
				allFields = append(allFields, field.Name)
			}
		}
	}

	return allFields, nil
}

// Items returns the field - value struct pairs as a map. obj can whether
// be a structure or pointer to structure.
func Items(obj interface{}) (map[string]interface{}, error) {
	return items(obj, false)
}

// ItemsDeep returns "flattened" items (fields from anonymous
// inner structs are treated as normal fields)
func ItemsDeep(obj interface{}) (map[string]interface{}, error) {
	return items(obj, true)
}

func items(obj interface{}, deep bool) (map[string]interface{}, error) {
	if !hasValidType(obj, []reflect.Kind{reflect.Struct, reflect.Ptr}) {
		return nil, errors.New("cannot use GetField on a non-struct interface")
	}

	objValue := ReflectValue(obj)
	objType := objValue.Type()
	fieldsCount := objType.NumField()

	allItems := make(map[string]interface{})

	for i := 0; i < fieldsCount; i++ {
		field := objType.Field(i)
		fieldValue := objValue.Field(i)
		if IsExportableField(field) {
			if deep && field.Anonymous {
				if m, err := items(fieldValue.Interface(), deep); err == nil {
					for k, v := range m {
						allItems[k] = v
					}
				} else {
					return nil, fmt.Errorf("cannot get items in %s: %s", field.Name, err.Error())
				}
			} else {
				allItems[field.Name] = fieldValue.Interface()
			}
		}
	}

	return allItems, nil
}

// Tags lists the struct tag fields. obj can whether
// be a structure or pointer to structure.
func Tags(obj interface{}, key string) (map[string]string, error) {
	return tags(obj, key, false)
}

// TagsDeep returns "flattened" tags (fields from anonymous
// inner structs are treated as normal fields)
func TagsDeep(obj interface{}, key string) (map[string]string, error) {
	return tags(obj, key, true)
}

func tags(obj interface{}, key string, deep bool) (map[string]string, error) {
	if !hasValidType(obj, []reflect.Kind{reflect.Struct, reflect.Ptr}) {
		return nil, errors.New("cannot use GetField on a non-struct interface")
	}

	objValue := ReflectValue(obj)
	objType := objValue.Type()
	fieldsCount := objType.NumField()

	allTags := make(map[string]string)

	for i := 0; i < fieldsCount; i++ {
		structField := objType.Field(i)
		if IsExportableField(structField) {
			if deep && structField.Anonymous {
				fieldValue := objValue.Field(i)
				if m, err := tags(fieldValue.Interface(), key, deep); err == nil {
					for k, v := range m {
						allTags[k] = v
					}
				} else {
					return nil, fmt.Errorf("cannot get items in %s: %s", structField.Name, err.Error())
				}
			} else {
				allTags[structField.Name] = structField.Tag.Get(key)
			}
		}
	}

	return allTags, nil
}

func ReflectValue(obj interface{}) reflect.Value {
	var val reflect.Value

	if reflect.TypeOf(obj).Kind() == reflect.Ptr {
		val = reflect.ValueOf(obj).Elem()
	} else {
		val = reflect.ValueOf(obj)
	}

	return val
}

func IsExportableField(field reflect.StructField) bool {
	// PkgPath is empty for exported fields.
	return field.PkgPath == ""
}

func hasValidType(obj interface{}, types []reflect.Kind) bool {
	for _, t := range types {
		if reflect.TypeOf(obj).Kind() == t {
			return true
		}
	}

	return false
}

func IsStruct(obj interface{}) bool {
	return reflect.TypeOf(obj).Kind() == reflect.Struct
}

func IsPointer(obj interface{}) bool {
	return reflect.TypeOf(obj).Kind() == reflect.Ptr
}

// HasMethod 对象是否有此方法
func HasMethod(obj interface{}, MethodName string) bool {
	ValueIface := reflect.ValueOf(obj)

	// Check if the passed interface is a pointer
	if ValueIface.Type().Kind() != reflect.Ptr {
		// Create a new type of obj, so we have a pointer to work with
		ValueIface = reflect.New(reflect.TypeOf(obj))
	}

	// Get the method by name
	Method := ValueIface.MethodByName(MethodName)
	return Method.IsValid()
}

var EfieldNameCountNotEqualToFieldValues = errors.New("field name count not equal to field values")

// SetFields 设置对象相应的值 obj.fieldNames0=fieldVals0, ...
func SetFields(obj interface{}, fieldNames []string, fieldVals []interface{}) (err error) {
	if len(fieldNames) > len(fieldVals) {
		return EfieldNameCountNotEqualToFieldValues
	}

	for i, fieldName := range fieldNames {
		errTmp := SetField(obj, fieldName, fieldVals[i])
		if errTmp != nil {
			err = errTmp
		}
	}

	return
}
