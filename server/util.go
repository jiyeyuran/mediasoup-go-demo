package main

import "encoding/json"

func NewBool(b bool) *bool {
	return &b
}

func Clone(dst, source interface{}) error {
	data, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
