package jsonx

import (
	"io"

	"encoding/json/jsontext"
	json "encoding/json/v2"
)

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v, json.Deterministic(true))
}

func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	b, err := Marshal(v)
	if err != nil {
		return nil, err
	}
	value := jsontext.Value(b)
	if err := value.Indent(jsontext.WithIndentPrefix(prefix), jsontext.WithIndent(indent)); err != nil {
		return nil, err
	}
	return []byte(value), nil
}

func Unmarshal(b []byte, v any) error {
	return json.Unmarshal(b, v)
}

func DecodeValue(b []byte) (any, error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func WriteIndented(w io.Writer, v any) error {
	b, err := MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}
