// Package jsonutil contains the JSON compatibility helpers used by config
// rendering.  Sing-box and Xray templates are commonly written as JSONC even
// though they are eventually decoded as JSON by the service.
package jsonutil

import "encoding/json"

// StripComments removes line and block comments from JSON while leaving
// strings untouched.  Whitespace is emitted in place of removed characters
// so that line and column information in a later JSON error remains useful.
func StripComments(data []byte) []byte {
	result := make([]byte, 0, len(data))
	inString := false
	escaped := false

	for i := 0; i < len(data); i++ {
		current := data[i]
		if inString {
			result = append(result, current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}

		if current == '"' {
			inString = true
			result = append(result, current)
			continue
		}

		if current != '/' || i+1 >= len(data) || (data[i+1] != '/' && data[i+1] != '*') {
			result = append(result, current)
			continue
		}

		block := data[i+1] == '*'
		result = append(result, ' ', ' ')
		i++
		for i+1 < len(data) {
			i++
			if block {
				if data[i] == '*' && i+1 < len(data) && data[i+1] == '/' {
					result = append(result, ' ', ' ')
					i++
					break
				}
			} else if data[i] == '\n' || data[i] == '\r' {
				result = append(result, data[i])
				break
			}
			if data[i] != '\n' && data[i] != '\r' {
				result = append(result, ' ')
			}
		}
	}

	return result
}

// Unmarshal decodes regular JSON as well as JSON containing // or /* */
// comments.  Comments are discarded before decoding.
func Unmarshal(data []byte, value any) error {
	return json.Unmarshal(StripComments(data), value)
}
