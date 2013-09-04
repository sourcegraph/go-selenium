package selenium

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestExecuteScript_Args(t *testing.T) {
	setup()
	defer teardown()

	input := map[string]interface{}{"script": "return 'foo'", "args": []interface{}{}}
	mux.HandleFunc("/session/123/execute", func(w http.ResponseWriter, r *http.Request) {
		var v map[string]interface{}
		json.NewDecoder(r.Body).Decode(&v)

		testMethod(t, r, "POST")
		testHeader(t, r, "content-type", "application/json")
		testHeader(t, r, "accept", "application/json")

		if !reflect.DeepEqual(v, input) {
			t.Errorf("Request body = %+v, want %+v", v, input)
		}

		fmt.Fprint(w, `{"status": 0, "value": "foo"}`)
	})

	result, err := client.ExecuteScript("return 'foo'", []interface{}{})
	if err != nil {
		t.Errorf("ExecuteScript returned error: %v", err)
	}

	want := "foo"
	if !reflect.DeepEqual(result, want) {
		t.Errorf("ExecuteScript returned %+v, want %+v", result, want)
	}
}

func TestExecuteScript_NoArgs(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/session/123/execute", func(w http.ResponseWriter, r *http.Request) {
		var v map[string]interface{}
		json.NewDecoder(r.Body).Decode(&v)

		args := []interface{}{}
		if !reflect.DeepEqual(v["args"], args) {
			t.Errorf("Args = %+v, want %+v", v["args"], args)
		}

		fmt.Fprint(w, `{"status": 0, "value": "foo"}`)
	})

	client.ExecuteScript("return 'foo'", nil)
}
