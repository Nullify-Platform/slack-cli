package output

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// PrintJSON marshals the value to indented JSON and prints to stdout.
// Uses omitempty struct tags and PruneEmpty for maps.
func PrintJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// PrintCompactJSON marshals with no indentation for piping.
func PrintCompactJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// PruneEmpty recursively removes nil, empty string, empty slice, empty map,
// and zero-value fields from the given value.
func PruneEmpty(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Map:
		result := make(map[string]interface{})
		for _, key := range val.MapKeys() {
			child := PruneEmpty(val.MapIndex(key).Interface())
			if child != nil && !isEmpty(child) {
				result[key.String()] = child
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case reflect.Slice:
		if val.IsNil() || val.Len() == 0 {
			return nil
		}
		var result []interface{}
		for i := 0; i < val.Len(); i++ {
			child := PruneEmpty(val.Index(i).Interface())
			if child != nil {
				result = append(result, child)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case reflect.String:
		if val.String() == "" {
			return nil
		}
		return v
	case reflect.Ptr, reflect.Interface:
		if val.IsNil() {
			return nil
		}
		return PruneEmpty(val.Elem().Interface())
	default:
		return v
	}
}

func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.String:
		return val.String() == ""
	case reflect.Slice, reflect.Map:
		return val.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return val.IsNil()
	default:
		return false
	}
}
