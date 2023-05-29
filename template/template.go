package template

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"
)

type Template struct {
	tmpl string
}

type Generator func(out io.Writer, obj interface{})

func ParseString(tmpl string) *Template {
	t, _ := Parse(strings.NewReader(tmpl))
	return t
}

func Parse(r io.Reader) (*Template, error) {
	bs, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &Template{
		tmpl: string(bs),
	}, nil
}

const (
	dot = iota
	name
)

type node struct {
	t     int
	left  *node
	right *node
	sval  string
}

type Funcmap map[string]func(w io.Writer) error

func (n *node) do(obj interface{}, funcs map[string]func(w io.Writer) error) interface{} {
	switch n.t {
	case dot:
		left := obj
		if n.left != nil {
			left = n.left.do(obj, funcs)
		}
		if n.right == nil {
			panic("Malformed Syntax Tree. Need a field name after a '.'")
		}
		if n.right.t != name {
			panic("Malformed Syntax Tree. Expected field name.")
		}
		v := reflect.ValueOf(left)
		if v.Kind() != reflect.Struct {
			panic(fmt.Sprintf("Invalid object. expected struct, but have %v", v.Kind()))
		}
		field := v.FieldByName(n.right.sval)
		// for i := 0; i < v.Type().NumField(); i++ {
		// 	fmt.Printf("FIELD %d: [%#v]\n", i, v.Type().Field(i))
		// }

		if field.Equal(reflect.Value{}) {
			panic(fmt.Sprintf("No such field [%s] in %#v", n.right.sval, v.Interface()))
		}
		return field.Interface()
	case name:
		f, ok := funcs[n.sval]
		if !ok {
			panic(fmt.Sprintf("No such function %s", n.sval))
		}
		return f
	}
	return nil
}

func parseSection(r *bufio.Reader) (*node, error) {
	n, err := parseSectRec(r)
	if err != nil {
		return n, err
	}
	if _, err := r.Peek(1); err == nil {
		bs, _ := io.ReadAll(r)
		return nil, fmt.Errorf("Failed to parse section. Junk at the end: \"%s\"", string(bs))

	}
	return n, nil
}

func discardWhitespace(r *bufio.Reader) {
	for rn, _, err := r.ReadRune(); err == nil; rn, _, err = r.ReadRune() {
		if rn != ' ' {
			break
		}
	}
	r.UnreadRune()
}

func parseSectRec(r *bufio.Reader) (*node, error) {
	var left *node
	var rn rune
	var err error
	for rn, _, err = r.ReadRune(); err == nil; rn, _, err = r.ReadRune() {
		if rn == ' ' {
			continue
		}
		if rn == '.' {
			sub, err := parseSectRec(r)
			if err != nil {
				return nil, err
			}
			left = &node{t: dot, left: left, right: sub}
			continue
		}
		if unicode.IsLetter(rn) {
			var sb strings.Builder
			sb.WriteRune(rn)
			for rn, _, err = r.ReadRune(); err == nil; rn, _, err = r.ReadRune() {
				if !(unicode.IsLetter(rn) || unicode.IsDigit(rn)) {
					break
				}
				sb.WriteRune(rn)
			}
			r.UnreadRune()
			discardWhitespace(r)
			if left != nil {
				return nil, fmt.Errorf("expected another dot or the end of the section, but got: %s", sb.String())
			}
			return &node{t: name, sval: sb.String()}, nil
		}
	}
	if left == nil && err != nil {
		return nil, err
	}
	return left, nil
}

func (t *Template) tmplSection(rs []rune, start int, out io.Writer, funcs map[string]func(w io.Writer) error, obj interface{}) (int, error) {
	var (
		end   int
		found bool
	)
	for i := start; i < len(t.tmpl); i++ {
		if rs[i] == rune('}') && i < len(rs)-1 && rs[i+1] == rune('}') {
			end = i + 2
			found = true
			break
		}
	}
	if !found {
		return 0, fmt.Errorf("Failed to find the end of the template insertion starting at character %d", start)
	}

	sect := rs[start : end-2]
	n, err := parseSection(bufio.NewReader(strings.NewReader(string(sect))))
	if err != nil {
		return 0, err
	}

	obj_out := n.do(obj, funcs)
	switch oo := obj_out.(type) {
	case func(io.Writer) error:
		err := oo(out)
		if err != nil {
			return 0, err
		}
	default:
		fmt.Fprintf(out, "%v", oo)
	}
	return end, nil
}

func (t *Template) Generate(out io.Writer, obj interface{}, funcs map[string]func(w io.Writer) error) error {
	rs := []rune(t.tmpl)
	for i := 0; i < len(rs); i++ {
		if rs[i] == rune('{') && i < len(rs)-1 && rs[i+1] == rune('{') {
			var err error
			i, err = t.tmplSection(rs, i+2, out, funcs, obj)
			if err != nil {
				return err
			}
		}
		fmt.Fprintf(out, "%c", rs[i])
	}
	return nil
}

func (t *Template) GenerateFn(obj interface{}, funcs map[string]func(w io.Writer) error) func(w io.Writer) error {
	return func(w io.Writer) error {
		return t.Generate(w, obj, funcs)
	}
}
