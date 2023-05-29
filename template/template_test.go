package template

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	for _, tt := range []struct {
		tmpl   string
		expect string
		o      interface{}
		funcs  map[string]func(w io.Writer) error
	}{
		{
			tmpl:   "<tag>{{ .X }}</tag>",
			expect: "<tag>10</tag>",
			o:      struct{ X int }{X: 10},
		},
		{
			tmpl:   "<html><body>{{ .X.Y.Z }}</body></html>",
			expect: "<html><body>hello</body></html>",
			o: struct {
				X struct {
					Y struct {
						Z string
					}
				}
			}{X: struct {
				Y struct {
					Z string
				}
			}{Y: struct {
				Z string
			}{Z: "hello"}}},
		},
		{
			tmpl:   "<tag>{{ .X }}</tag><othertag>{{ myfunc }}</othertag>",
			expect: "<tag>10</tag><othertag>happy</othertag>",
			o:      struct{ X int }{X: 10},
			funcs: map[string]func(w io.Writer) error{
				"myfunc": func(w io.Writer) error {
					fmt.Fprintf(w, "happy")
					return nil
				},
			},
		},
	} {
		t.Run(tt.tmpl, func(t *testing.T) {
			tpl := ParseString(tt.tmpl)
			var sb strings.Builder
			tpl.Generate(&sb, tt.o, tt.funcs)

			out := sb.String()
			if out != tt.expect {
				t.Fatalf("Expected [%s] but got [%s]", tt.expect, out)
			}
		})
	}
}

func TestParseSection(t *testing.T) {
	for _, tt := range []struct {
		in  string
		out *node
		err error
	}{
		{
			in:  "myfunc",
			out: &node{t: name, sval: "myfunc"},
		},
		{
			in:  "    myfunc     ",
			out: &node{t: name, sval: "myfunc"},
		},
		{
			in:  " . X ",
			out: &node{t: dot, right: &node{t: name, sval: "X"}},
		},
		{
			in: " .X.Y.Z",
			out: &node{
				t: dot,
				left: &node{
					t: dot,
					left: &node{
						t:     dot,
						right: &node{t: name, sval: "X"},
					},
					right: &node{t: name, sval: "Y"},
				},
				right: &node{t: name, sval: "Z"},
			},
		},
		{
			in:  ".x myfunc .y .z",
			err: errors.New("expected another dot or the end of the section, but got: myfunc"),
		},

		{
			in:  "foo bar",
			err: errors.New("Failed to parse section. Junk at the end: \"bar\""),
		},
	} {
		t.Run(tt.in, func(t *testing.T) {
			rdr := bufio.NewReader(strings.NewReader(tt.in))
			n, err := parseSection(rdr)
			if err != nil {
				if tt.err != nil {
					if err.Error() != tt.err.Error() {
						t.Fatalf("Expected error [%v] but got [%v].", tt.err, err)
					}
				} else {
					t.Fatalf("Expected no error, but got: [%v]", err)
				}
			} else if tt.err != nil {
				t.Fatalf("Expected error [%v], but got nil.", tt.err)
			}
			assert.Equal(t, tt.out, n)
		})

	}
}
