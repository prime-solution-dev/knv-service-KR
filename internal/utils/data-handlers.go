package utils

import (
	"reflect"
)

func ExtractUpdates(entities []interface{}) ([]map[string]interface{}, error) {
	var result []map[string]interface{}

	for _, entity := range entities {
		val := reflect.ValueOf(entity)

		if val.Kind() == reflect.Slice {
			for i := 0; i < val.Len(); i++ {
				elem := val.Index(i)
				if elem.Kind() == reflect.Struct {
					typ := elem.Type()

					updates := map[string]interface{}{}
					for i := 0; i < elem.NumField(); i++ {
						field := elem.Field(i)
						fieldName := typ.Field(i).Name

						if field.Kind() == reflect.Ptr && !field.IsNil() {
							updates[fieldName] = field.Elem().Interface()
						} else if field.Kind() != reflect.Ptr {
							updates[fieldName] = field.Interface()
						}
					}
					result = append(result, updates)
				}
			}
		}
	}

	return result, nil
}
