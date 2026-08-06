package focus

import (
	"encoding/json"
	"reflect"
	"strings"
)

func (r Record) MarshalJSON() ([]byte, error) {
	type alias Record
	base, err := json.Marshal(alias(r))
	if err != nil {
		return nil, err
	}
	if len(r.Extensions) == 0 {
		return base, nil
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(base, &merged); err != nil {
		return nil, err
	}
	for k, v := range r.Extensions {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		merged[k] = raw
	}
	return json.Marshal(merged)
}

func (r *Record) UnmarshalJSON(data []byte) error {
	type alias Record
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*r = Record(a)

	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	known := columnNames()
	ext := make(map[string]any)
	for k, v := range all {
		if known[k] {
			continue
		}
		var val any
		if err := json.Unmarshal(v, &val); err != nil {
			return err
		}
		ext[k] = val
	}
	if len(ext) > 0 {
		r.Extensions = ext
	}
	return nil
}

func columnNames() map[string]bool {
	t := reflect.TypeOf(Record{})
	names := make(map[string]bool, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name != "" {
			names[name] = true
		}
	}
	return names
}
