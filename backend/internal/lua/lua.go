// Package lua parses the subset of Lua used in DCS data files such as
// debrief.log: a sequence of `name = value` assignments where values are
// numbers, strings, booleans, nil or nested tables.
//
// It is deliberately small and dependency-free. It does not execute Lua; it only
// reads data, so it is safe to point at any file DCS produces.
package lua

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Parse reads a Lua data document and returns its top-level assignments. Tables
// become map[string]any when keyed and []any when they look like arrays (keys
// 1..n in order).
func Parse(data []byte) (map[string]any, error) {
	p := &parser{src: string(data)}
	root := map[string]any{}

	for {
		p.skipSpace()
		if p.eof() {
			break
		}
		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.consume('=') {
			return nil, p.errorf("expected '=' after %q", key)
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		root[key] = val
		p.consume(',')
		p.consume(';')
	}
	return root, nil
}

type parser struct {
	src string
	pos int
}

func (p *parser) eof() bool { return p.pos >= len(p.src) }

func (p *parser) errorf(format string, args ...any) error {
	line := 1 + strings.Count(p.src[:min(p.pos, len(p.src))], "\n")
	return fmt.Errorf("lua: line %d: %s", line, fmt.Sprintf(format, args...))
}

func (p *parser) skipSpace() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '-' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '-' {
			// Comment to end of line.
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			p.pos++
			continue
		}
		return
	}
}

func (p *parser) consume(c byte) bool {
	if p.pos < len(p.src) && p.src[p.pos] == c {
		p.pos++
		return true
	}
	return false
}

// parseKey reads a table key: an identifier, a [expr] index, or a string.
func (p *parser) parseKey() (string, error) {
	p.skipSpace()
	if p.eof() {
		return "", p.errorf("unexpected end of input, expected a key")
	}
	if p.src[p.pos] == '[' {
		p.pos++
		v, err := p.parseValue()
		if err != nil {
			return "", err
		}
		p.skipSpace()
		if !p.consume(']') {
			return "", p.errorf("expected ']'")
		}
		return keyString(v), nil
	}
	if p.src[p.pos] == '"' || p.src[p.pos] == '\'' {
		v, err := p.parseString()
		if err != nil {
			return "", err
		}
		return v, nil
	}
	start := p.pos
	for p.pos < len(p.src) {
		c := rune(p.src[p.pos])
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' {
			p.pos++
			continue
		}
		break
	}
	if start == p.pos {
		return "", p.errorf("invalid key starting with %q", string(p.src[p.pos]))
	}
	return p.src[start:p.pos], nil
}

func (p *parser) parseValue() (any, error) {
	p.skipSpace()
	if p.eof() {
		return nil, p.errorf("unexpected end of input, expected a value")
	}
	// DCS data files wrap localised strings in the gettext call _("..."), e.g.
	// callsign = {{["common"] = {_("OMAA"), "OMAA"}}}. Treat it as its argument.
	if strings.HasPrefix(p.src[p.pos:], "_(") {
		p.pos += 2
		p.skipSpace()
		// A truncated file can end right here; parseString would index past the
		// end. This is the only caller that does not guarantee a quote.
		if p.eof() {
			return nil, p.errorf("expected a quoted string after _(")
		}
		v, err := p.parseString()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.consume(')') {
			return nil, p.errorf("expected ')' to close _(")
		}
		return v, nil
	}
	switch c := p.src[p.pos]; {
	case c == '{':
		return p.parseTable()
	case c == '"' || c == '\'':
		return p.parseString()
	case c == '-' || c == '+' || (c >= '0' && c <= '9'):
		return p.parseNumber()
	case strings.HasPrefix(p.src[p.pos:], "true"):
		p.pos += 4
		return true, nil
	case strings.HasPrefix(p.src[p.pos:], "false"):
		p.pos += 5
		return false, nil
	case strings.HasPrefix(p.src[p.pos:], "nil"):
		p.pos += 3
		return nil, nil
	}
	// A bare identifier is an enum constant from DCS data files
	// (BEACON_TYPE_VOR_DME, MODULATIONTYPE_AM, VHF_HI…). It is returned as its
	// own name, which is what the callers need to tell beacon types apart.
	if c := p.src[p.pos]; isIdentChar(rune(c)) {
		start := p.pos
		for p.pos < len(p.src) && isIdentChar(rune(p.src[p.pos])) {
			p.pos++
		}
		return p.src[start:p.pos], nil
	}
	return nil, p.errorf("unexpected character %q", string(p.src[p.pos]))
}

// isIdentChar reports whether c can appear in a Lua identifier.
func isIdentChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_'
}

func (p *parser) parseTable() (any, error) {
	p.pos++ // consume '{'

	object := map[string]any{}
	// array collects only the values written under explicit numeric keys that
	// happen to be exactly the implicit position 1..n. A table whose entries are
	// all of that form collapses to a slice (debrief `events`, `world_state`).
	// A table that also uses bare values stays a numeric-keyed map, which is the
	// shape DCS's own radio.lua/beacons.lua require.
	var array []any
	arrayNext := 1
	isArray := true

	for {
		p.skipSpace()
		if p.eof() {
			return nil, p.errorf("unterminated table")
		}
		if p.consume('}') {
			break
		}

		// An entry is either `key = value` or a bare value (implicit index).
		save := p.pos
		key, keyErr := p.parseKey()
		p.skipSpace()
		if keyErr == nil && p.consume('=') {
			v, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			object[key] = v
			if n, ok := numericKey(key); ok && n == arrayNext {
				array = append(array, v)
				arrayNext++
			} else {
				isArray = false
			}
		} else {
			p.pos = save
			v, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			// A bare value is not an explicit [n], so the table is not the
			// slice form. Store it under the first free numeric slot instead of
			// blindly renumbering later, so an explicit key the value would have
			// overwritten is preserved.
			isArray = false
			slot := arrayNext
			for {
				if _, taken := object[strconv.Itoa(slot)]; !taken {
					break
				}
				slot++
			}
			object[strconv.Itoa(slot)] = v
			arrayNext = slot + 1
		}

		p.skipSpace()
		p.consume(',')
		p.consume(';')
	}

	// A pure array (explicit keys exactly 1..n) becomes a slice.
	if isArray && len(object) == len(array) {
		return array, nil
	}
	if len(object) == 0 {
		return []any{}, nil
	}
	return object, nil
}

func (p *parser) parseString() (string, error) {
	quote := p.src[p.pos]
	p.pos++
	var sb strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch c {
		case quote:
			p.pos++
			return sb.String(), nil
		case '\\':
			p.pos++
			if p.pos >= len(p.src) {
				return "", p.errorf("unterminated escape")
			}
			e := p.src[p.pos]
			switch e {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			case '\'':
				sb.WriteByte('\'')
			default:
				if e >= '0' && e <= '9' {
					// \ddd decimal escape. Lua rejects a value above 255; byte(n)
					// would silently wrap, so the escape is reported instead.
					n := 0
					for k := 0; k < 3 && p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9'; k++ {
						n = n*10 + int(p.src[p.pos]-'0')
						p.pos++
					}
					p.pos-- // compensate the loop increment below
					if n > 255 {
						return "", p.errorf("decimal escape \\%d is out of range", n)
					}
					sb.WriteByte(byte(n))
				} else {
					sb.WriteByte(e)
				}
			}
			p.pos++
		default:
			sb.WriteByte(c)
			p.pos++
		}
	}
	return "", p.errorf("unterminated string")
}

func (p *parser) parseNumber() (any, error) {
	start := p.pos
	if p.pos < len(p.src) && (p.src[p.pos] == '-' || p.src[p.pos] == '+') {
		p.pos++
	}
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' ||
			((c == '-' || c == '+') && p.pos > start && (p.src[p.pos-1] == 'e' || p.src[p.pos-1] == 'E')) {
			p.pos++
			continue
		}
		break
	}
	text := p.src[start:p.pos]
	if n, err := strconv.ParseInt(text, 10, 64); err == nil {
		return float64(n), nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(f) {
		return nil, p.errorf("invalid number %q", text)
	}
	return f, nil
}

func keyString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprint(t)
	}
}

func numericKey(k string) (int, bool) {
	n, err := strconv.Atoi(k)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
