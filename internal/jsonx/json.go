package jsonx

import (
	"bytes"
	"fmt"
	"io"

	"encoding/json/jsontext"
	json "encoding/json/v2"
)

// Number is a JSON number literal preserved from source text.
type Number string

// Member is one JSON object member in source order.
type Member struct {
	Name  string
	Value any
}

// Object is a JSON object value that preserves source member order.
type Object []Member

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

// DecodeValue materializes a JSON value tree while preserving number literals.
// It intentionally mirrors the token walk in etch's jsonSpanDecoder only at the
// syntax layer: jsonSpanDecoder records source byte spans for localized edits,
// while DecodeValue discards positions and keeps just semantic values.
func DecodeValue(b []byte) (any, error) {
	dec := jsontext.NewDecoder(bytes.NewReader(b), jsontext.AllowDuplicateNames(true))
	v, err := readValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.ReadToken(); err == io.EOF {
		return v, nil
	} else if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("JSON input contains trailing data")
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

func (n Number) MarshalJSON() ([]byte, error) {
	b := []byte(n)
	if err := validateNumber(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (o Object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	enc := jsontext.NewEncoder(&b, jsontext.AllowDuplicateNames(true))
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return nil, err
	}
	for _, member := range o {
		if err := enc.WriteToken(jsontext.String(member.Name)); err != nil {
			return nil, err
		}
		value, err := Marshal(member.Value)
		if err != nil {
			return nil, err
		}
		if err := enc.WriteValue(jsontext.Value(value)); err != nil {
			return nil, err
		}
	}
	if err := enc.WriteToken(jsontext.EndObject); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

func validateNumber(b []byte) error {
	dec := jsontext.NewDecoder(bytes.NewReader(b))
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if tok.Kind() != '0' {
		return fmt.Errorf("invalid JSON number %q", string(b))
	}
	if _, err := dec.ReadToken(); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("invalid JSON number %q", string(b))
}

func readValue(dec *jsontext.Decoder) (any, error) {
	tok, err := dec.ReadToken()
	if err != nil {
		return nil, err
	}
	switch tok.Kind() {
	case 'n':
		return nil, nil
	case 'f', 't':
		return tok.Bool(), nil
	case '"':
		return tok.String(), nil
	case '0':
		return Number(tok.String()), nil
	case '{':
		return readObject(dec)
	case '[':
		return readArray(dec)
	default:
		return nil, fmt.Errorf("unexpected JSON token %s", tok.Kind())
	}
}

func readObject(dec *jsontext.Decoder) (Object, error) {
	var members Object
	for {
		switch dec.PeekKind() {
		case '}':
			_, err := dec.ReadToken()
			return members, err
		case '"':
		default:
			if _, err := dec.ReadToken(); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("object member name must be a string")
		}
		keyTok, err := dec.ReadToken()
		if err != nil {
			return nil, err
		}
		key := keyTok.String()
		value, err := readValue(dec)
		if err != nil {
			return nil, err
		}
		members = append(members, Member{Name: key, Value: value})
	}
}

func readArray(dec *jsontext.Decoder) ([]any, error) {
	var out []any
	for {
		if dec.PeekKind() == ']' {
			_, err := dec.ReadToken()
			return out, err
		}
		value, err := readValue(dec)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
}
