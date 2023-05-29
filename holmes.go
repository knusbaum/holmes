package holmes

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	htmpl "github.com/knusbaum/holmes/template"
)

var structTmpl = `
<div>
<p>struct {{ .Name }}{</p>
{{ structFields }}
<p>}</p>
</div>
`

var structFields = `
<div>
<p>{{ .Name }} {{ .Type }} : {{ summary }}</p> 
</div>
`

type tmplStructField struct {
	Name    string
	Type    string
	ValueFn func() template.HTML
}

// Inspector is an optional interface. If an object implements Inspector, it has
// complete control over its rendering. It also has complete control over its subpaths.
// It may choose to pass control to them when len(path) > 0, or do something else.
type Inspector interface {
	ObjectHTML(prefix string, path []string, w io.Writer, r *http.Request) error
}

// Summarizer is an optional interface. If an object implements Summarizer, it can control
// how its object summary is rendered.
// SummaryHTML should write a short HTML summary describing the object to w, usually with
// a link to prefix, which is the path to the object.
type Summarizer interface {
	SummaryHTML(prefix string, w io.Writer, r *http.Request) error
}

// SummaryHTML returns a function that will write a summary of obj to w. This is useful
// for writing object summaries with template functions.
func SummaryHTML(obj interface{}, prefix string, r *http.Request) func(w io.Writer) error {
	if i, ok := obj.(Summarizer); ok {
		return func(w io.Writer) error {
			i.SummaryHTML(prefix, w, r)
			return nil
		}
	}
	return func(w io.Writer) error {
		_, err := fmt.Fprintf(w, `<a href="%s">%#v</a>`, prefix, obj)
		return err
	}
}

// ObjectHTMLFn returns a function that will call ObjectHTML on obj, prefix, path, and r.
// This is useful in templates, as a template function.
func ObjectHTMLFn(obj interface{}, prefix string, path []string, r *http.Request) func(w io.Writer) error {
	return func(w io.Writer) error {
		ObjectHTML(obj, prefix, path, w, r)
		return nil
	}
}

// NextPathHTML Calls ObjectHTML on the field named by path[0] in obj. This is useful when implementing
// the Inspector interface, for dispatching to sub-objects the same way holmes regularly does when
// an object does not implement the Inspector interface.
//
// Usual use looks like this:
//
//	func (o *MyObject) ObjectHTML(prefix string, path []string, w io.Writer, r *http.Request) error {
//		if len(path) > 0 {
//			return holmes.NextPathHTML(o, prefix, path, w, r)
//		}
//		...
//	}
func NextPathHTML(obj interface{}, prefix string, path []string, w io.Writer, r *http.Request) error {
	v := reflect.ValueOf(obj)
	switch v.Kind() {
	case reflect.Struct:
		field := v.FieldByName(path[0])
		fmt.Printf("FIELD %s: VALUE: %#v\n", path[0], field)
		if field.Equal(reflect.Value{}) {
			fmt.Printf("IT'S NOT VALID!\n")
			return fmt.Errorf("Invalid path. No such field %s in type %s", path[0], v.Type().String())
		}
		fmt.Printf("CONTINUED ANYWAY!\n")
		return ObjectHTML(field.Interface(), Subpath(prefix, path[0]), path[1:], w, r)
	case reflect.Pointer:
		if v.IsZero() {
			fmt.Fprintf(w, "<div><p>nil</p></div>")
			return nil
		}
		return NextPathHTML(v.Elem().Interface(), prefix, path, w, r)
	default:
		fmt.Errorf("Cannot handle subobject from type %v\n", v.Kind().String())
	}
	return fmt.Errorf("Cannot handle subobject from type %v\n", v.Kind().String())
}

// ObjectHTML renders html for obj to w, at the path prefix. If the URL does not end at prefix, the
// remainder of the path, split on '/', is specified in path.
func ObjectHTML(obj interface{}, prefix string, path []string, w io.Writer, r *http.Request) error {
	if i, ok := obj.(Inspector); ok {
		fmt.Printf("OBJECT: %#v\n", obj)
		return i.ObjectHTML(prefix, path, w, r)
	}
	v := reflect.ValueOf(obj)
	switch v.Kind() {
	case reflect.Struct:
		return handleStruct(v, prefix, path, w, r)
	case reflect.Pointer:
		if v.IsZero() {
			fmt.Fprintf(w, "<div><p>nil</p></div>")
			return nil
		}
		return ObjectHTML(v.Elem().Interface(), prefix, path, w, r)
	case reflect.Slice:
		return handleSlice(v, prefix, path, w, r)
	default:
		_, err := fmt.Fprintf(w, "%#v %s", obj, v.Type().Name())
		return err
	}
}

func handleSlice(v reflect.Value, prefix string, path []string, w io.Writer, r *http.Request) error {
	if len(path) > 0 {
		fmt.Printf("PATH: %#v\n", path)
		i, err := strconv.Atoi(path[0])
		if err != nil {
			return fmt.Errorf("Cannot take subpath %s from %s, expected integer.", path[0], prefix)
		}
		len := v.Len()
		if i >= v.Len() {
			return fmt.Errorf("Cannot take subpath %s from %s: object contains only %d elements.\n", path[0], prefix, len)
		}
		o := v.Index(i)
		ObjectHTML(o.Interface(), Subpath(prefix, path[0]), path[1:], w, r)
		return nil
	}
	fmt.Fprintf(w, "<div><p>%s</p>", v.Type().String())
	for i := 0; i < v.Len(); i++ {
		fmt.Fprintf(w, "<p>%d: ", i)
		SummaryHTML(v.Index(i).Interface(), Subpath(prefix, strconv.Itoa(i)), r)(w)
		fmt.Fprintf(w, "</p>")
	}
	fmt.Fprintf(w, "</div>")
	return nil
}

func handleStruct(v reflect.Value, prefix string, path []string, w io.Writer, r *http.Request) error {
	if len(path) > 0 {
		fmt.Printf("PATH: %#v\n", path)
		field := v.FieldByName(path[0])
		return ObjectHTML(field.Interface(), Subpath(prefix, path[0]), path[1:], w, r)
	}

	t := htmpl.ParseString(structTmpl)
	ft := htmpl.ParseString(structFields)
	typ := v.Type()

	tdat := struct{ Name string }{typ.Name()}

	return t.Generate(w, tdat, htmpl.Funcmap{
		"structFields": func(w io.Writer) error {
			for i := 0; i < v.NumField(); i++ {
				field := typ.Field(i)
				fdat := struct {
					Name string
					Type string
				}{field.Name, field.Type.Name()}

				err := ft.Generate(w, fdat, htmpl.Funcmap{
					"summary": SummaryHTML(v.Field(i).Interface(), Subpath(prefix, field.Name), r),
				})
				if err != nil {
					return err
				}

			}
			return nil
		}})
}

// Subpath joins a path prefix with a new part, separated by a forward-slash.
func Subpath(prefix, part string) string {
	if strings.HasSuffix(prefix, "/") {
		prefix = prefix[:len(prefix)-1]
	}
	if strings.HasPrefix(part, "/") {
		part = part[1:]
	}
	return fmt.Sprintf("%s/%s", prefix, part)
}

// Handler returns a new holmes handler which will render obj at the prefix.
// prefix must match the path that the handler is registered at, and must end with a slash.
//
// Example:
//
//	http.HandleFunc("/my/path/to/my/object/", holmes.Handler("/my/path/to/my/object/", obj))
func Handler(prefix string, obj interface{}) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")
		k := 0
		for i := 0; i < len(path); i++ {
			if path[i] == "" {
				continue
			}
			path[k] = path[i]
			k++
		}
		path = path[:k]

		//prefix := "/"
		err := ObjectHTML(obj, prefix, path, w, r)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}
