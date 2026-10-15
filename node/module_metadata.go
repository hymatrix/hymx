package node

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	goarSchema "github.com/permadao/goar/schema"
)

// readModuleTags reads metadata without materializing the module's image payload.
// BundleItem normally writes tags before data, so the common path stops before
// reading data at all. Other field orders are supported with bounded buffering.
// Payload integrity remains the responsibility of the runtime consuming it.
func readModuleTags(src io.Reader) ([]goarSchema.Tag, error) {
	r := bufio.NewReaderSize(src, 32<<10)
	if c, err := jsonNonSpace(r); err != nil || c != '{' {
		return nil, fmt.Errorf("module metadata must be a JSON object")
	}
	for {
		c, err := jsonNonSpace(r)
		if err != nil {
			return nil, err
		}
		if c == '}' {
			return nil, fmt.Errorf("module tags not found")
		}
		if c != '"' {
			return nil, fmt.Errorf("expected module field name")
		}
		key, err := moduleJSONString(r, true)
		if err != nil {
			return nil, err
		}
		if c, err = jsonNonSpace(r); err != nil || c != ':' {
			return nil, fmt.Errorf("expected colon after module field")
		}
		if strings.EqualFold(key, "tags") {
			var tags []goarSchema.Tag
			// Tags are small; reject oversized metadata instead of allowing an
			// unbounded allocation through this otherwise memory-bounded path.
			err := json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&tags)
			return tags, err
		}
		if err := skipModuleJSONValue(r, 0); err != nil {
			return nil, err
		}
		if c, err = jsonNonSpace(r); err != nil {
			return nil, err
		} else if c == '}' {
			return nil, fmt.Errorf("module tags not found")
		} else if c != ',' {
			return nil, fmt.Errorf("expected comma after module field")
		}
	}
}

func jsonNonSpace(r *bufio.Reader) (byte, error) {
	for {
		c, err := r.ReadByte()
		if err != nil || (c != ' ' && c != '\t' && c != '\r' && c != '\n') {
			return c, err
		}
	}
}

// moduleJSONString starts immediately after the opening quote. Large ignored
// strings are scanned without retaining their contents.
func moduleJSONString(r *bufio.Reader, capture bool) (string, error) {
	var raw []byte
	if capture {
		raw = append(raw, '"')
	}
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if capture {
			raw = append(raw, c)
			if len(raw) > 64<<10 {
				return "", fmt.Errorf("module field name too long")
			}
		}
		if c == '"' {
			if !capture {
				return "", nil
			}
			var value string
			err := json.Unmarshal(raw, &value)
			return value, err
		}
		if c < 0x20 {
			return "", fmt.Errorf("control byte in module JSON string")
		}
		if c != '\\' {
			continue
		}
		escape, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if capture {
			raw = append(raw, escape)
		}
		switch escape {
		case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		case 'u':
			for i := 0; i < 4; i++ {
				h, err := r.ReadByte()
				if err != nil {
					return "", err
				}
				if !((h >= '0' && h <= '9') || (h >= 'a' && h <= 'f') || (h >= 'A' && h <= 'F')) {
					return "", fmt.Errorf("invalid Unicode escape in module JSON")
				}
				if capture {
					raw = append(raw, h)
				}
			}
		default:
			return "", fmt.Errorf("invalid escape in module JSON")
		}
	}
}

func skipModuleJSONValue(r *bufio.Reader, depth int) error {
	if depth > 128 {
		return fmt.Errorf("module metadata nesting too deep")
	}
	c, err := jsonNonSpace(r)
	if err != nil {
		return err
	}
	switch c {
	case '"':
		_, err := moduleJSONString(r, false)
		return err
	case '{', '[':
		end := byte(']')
		object := c == '{'
		if object {
			end = '}'
		}
		c, err = jsonNonSpace(r)
		if err != nil || c == end {
			return err
		}
		if err := r.UnreadByte(); err != nil {
			return err
		}
		for {
			if object {
				if c, err = jsonNonSpace(r); err != nil || c != '"' {
					return fmt.Errorf("expected nested module field name")
				}
				if _, err := moduleJSONString(r, false); err != nil {
					return err
				}
				if c, err = jsonNonSpace(r); err != nil || c != ':' {
					return fmt.Errorf("expected nested module field colon")
				}
			}
			if err := skipModuleJSONValue(r, depth+1); err != nil {
				return err
			}
			if c, err = jsonNonSpace(r); err != nil || c == end {
				return err
			} else if c != ',' {
				return fmt.Errorf("expected module value separator")
			}
		}
	default:
		// JSON numbers and literals are small. Delimiters stay in the stream
		// for the enclosing object or array to consume.
		raw := []byte{c}
		for {
			c, err = r.ReadByte()
			if err != nil {
				return err
			}
			if strings.ContainsRune(" \t\r\n,}]", rune(c)) {
				if err := r.UnreadByte(); err != nil {
					return err
				}
				break
			}
			raw = append(raw, c)
			if len(raw) > 128 {
				return fmt.Errorf("module JSON scalar too long")
			}
		}
		if !json.Valid(raw) {
			return fmt.Errorf("invalid module JSON scalar")
		}
		return nil
	}
}
