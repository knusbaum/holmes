package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/knusbaum/holmes"
	"github.com/knusbaum/holmes/template"
)

type Foo struct {
	Jar    int
	Baz    int
	Quux   string
	Bar    *Bar
	Ints   []int
	Things []interface{}
	Toot   interface{}
}

type Bar struct {
	Baz int
	Boo string
}

type Doot struct {
	X, Y, Z int
}

func (b *Bar) SummaryHTML(prefix string, w io.Writer, r *http.Request) error {
	_, err := fmt.Fprintf(w, `<a href="%s">[Hello, I'm a Bar and this is my custom summary!]</a>`, prefix)
	return err
}

func (f *Foo) ObjectHTML(prefix string, path []string, w io.Writer, r *http.Request) error {
	if len(path) > 0 {
		// switch path[0] {
		// case "Jar":
		// 	holmes.ObjectHTML(f.Jar, holmes.Subpath(prefix, path[0]), path[1:], w, r)
		// case "Baz":
		// 	holmes.ObjectHTML(f.Baz, holmes.Subpath(prefix, path[0]), path[1:], w, r)
		// case "Quux":
		// 	holmes.ObjectHTML(f.Quux, holmes.Subpath(prefix, path[0]), path[1:], w, r)
		// case "Bar":
		// 	holmes.ObjectHTML(f.Bar, holmes.Subpath(prefix, path[0]), path[1:], w, r)
		// case "Toot":
		// 	holmes.ObjectHTML(f.Toot, holmes.Subpath(prefix, path[0]), path[1:], w, r)
		// }
		return holmes.NextPathHTML(f, prefix, path, w, r)
	}
	tmpl := `
<div>
<p>struct Foo {</p>
<p>&nbsp;Jar int : {{ jarsummary }}</p>
<p>&nbsp;Baz int : {{ bazsummary }}</p>
<p>&nbsp;Quux string : {{ quuxsummary }}</p>
<p>&nbsp;Ints []int : {{ intssummary }}</p>
<p>&nbsp;Things []any : {{ thingssummary }}</p>
<p>&nbsp;Bar *Bar : {{ barsummary }}</p>
<p>&nbsp;Toot interface{} : {{ tootsummary }}</p>
<p>}</p>
</div>
`
	t := template.ParseString(tmpl)
	t.Generate(w, f, template.Funcmap{
		"jarsummary":  holmes.SummaryHTML(f.Jar, holmes.Subpath(prefix, "Jar"), r),
		"bazsummary":  holmes.SummaryHTML(f.Baz, holmes.Subpath(prefix, "Baz"), r),
		"quuxsummary": holmes.SummaryHTML(f.Quux, holmes.Subpath(prefix, "Quux"), r),
		"intssummary": holmes.SummaryHTML(f.Ints, holmes.Subpath(prefix, "Ints"), r),
		"barsummary":  holmes.SummaryHTML(f.Bar, holmes.Subpath(prefix, "Bar"), r),
		"tootsummary": holmes.SummaryHTML(f.Toot, holmes.Subpath(prefix, "Toot"), r),
		"thingssummary": func(w io.Writer) error {
			_, err := fmt.Fprintf(w, `<a href="%s">[Just a bunch of things (%d items)]</a>`,
				holmes.Subpath(prefix, "Things"),
				len(f.Things))
			return err
		},
	})
	return nil
}

func main() {
	http.HandleFunc("/oop/", holmes.Handler("/oop/", &Foo{
		Bar:  &Bar{},
		Ints: []int{1, 2, 3, 4, 5, 6, 7},
		Things: []interface{}{
			&Foo{},
			&Doot{},
			1, 2, 3,
		},
		Toot: &Doot{1, 2, 3}}))
	log.Fatal(http.ListenAndServe(":8083", nil))
}
