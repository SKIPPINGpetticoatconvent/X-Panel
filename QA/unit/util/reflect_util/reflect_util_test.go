package reflect_util_test

import (
	"reflect"
	"testing"

	"x-ui/util/reflect_util"
)

type TestStruct struct {
	Field1 string
	Field2 int
	Field3 bool
}

func TestGetFields(t *testing.T) {
	ts := TestStruct{}
	typ := reflect.TypeOf(ts)
	fields := reflect_util.GetFields(typ)

	if len(fields) != 3 {
		t.Errorf("GetFields returned %d fields, want 3", len(fields))
	}

	expectedNames := []string{"Field1", "Field2", "Field3"}
	for i, f := range fields {
		if f.Name != expectedNames[i] {
			t.Errorf("Field %d name = %s, want %s", i, f.Name, expectedNames[i])
		}
	}
}

func TestGetFieldValues(t *testing.T) {
	ts := TestStruct{
		Field1: "hello",
		Field2: 42,
		Field3: true,
	}
	val := reflect.ValueOf(ts)
	values := reflect_util.GetFieldValues(val)

	if len(values) != 3 {
		t.Errorf("GetFieldValues returned %d values, want 3", len(values))
	}

	if values[0].String() != "hello" {
		t.Errorf("Field1 value = %v, want hello", values[0])
	}
	if values[1].Int() != 42 {
		t.Errorf("Field2 value = %v, want 42", values[1])
	}
	if values[2].Bool() != true {
		t.Errorf("Field3 value = %v, want true", values[2])
	}
}
