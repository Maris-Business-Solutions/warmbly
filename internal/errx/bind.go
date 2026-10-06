package errx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// maxBindProblems caps how many validation failures one refusal lists.
const maxBindProblems = 5

// Validation failures name fields as the caller wrote them (the json key), not as Go does.
func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(jsonFieldName)
	}
	binding.Validator = rejectingTextValidator{delegate: binding.Validator}
}

type rejectingTextValidator struct {
	delegate binding.StructValidator
}

func (v rejectingTextValidator) ValidateStruct(value any) error {
	if err := v.delegate.ValidateStruct(value); err != nil {
		return err
	}
	return unsupportedText(reflect.ValueOf(value), "")
}

func (v rejectingTextValidator) Engine() any {
	return v.delegate.Engine()
}

type unsupportedTextError struct {
	field string
}

func (e *unsupportedTextError) Error() string {
	return fmt.Sprintf("field %q contains a NUL character", e.field)
}

func unsupportedText(value reflect.Value, path string) error {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.String:
		if strings.ContainsRune(value.String(), '\x00') {
			if path == "" {
				path = "value"
			}
			return &unsupportedTextError{field: path}
		}
	case reflect.Struct:
		typeOf := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := typeOf.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := boundFieldName(field)
			if name == "" {
				continue
			}
			if err := unsupportedText(value.Field(i), joinFieldPath(path, name)); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := unsupportedText(value.Index(i), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			if err := unsupportedText(iter.Key(), joinFieldPath(path, "key")); err != nil {
				return err
			}
			key := fmt.Sprint(iter.Key().Interface())
			if err := unsupportedText(iter.Value(), joinFieldPath(path, key)); err != nil {
				return err
			}
		}
	}
	return nil
}

func boundFieldName(field reflect.StructField) string {
	for _, tag := range []string{"json", "form"} {
		name, _, _ := strings.Cut(field.Tag.Get(tag), ",")
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
	}
	return field.Name
}

func joinFieldPath(parent, field string) string {
	if parent == "" {
		return field
	}
	return parent + "." + field
}

func jsonFieldName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	if name == "-" {
		return ""
	}
	return name
}

// InvalidBody is the 400 for a request body that failed to bind, naming what
// was wrong with it: empty, not JSON, the wrong shape, or missing a field.
func InvalidBody(err error) *Error {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
		sizeErr   *http.MaxBytesError
		timeErr   *time.ParseError
		textErr   *unsupportedTextError
	)
	switch {
	case err == nil:
		return ErrInvalid
	case errors.Is(err, io.EOF):
		return New(BadRequest, "The request body is empty. Send a JSON body with Content-Type: application/json.")
	case errors.Is(err, io.ErrUnexpectedEOF):
		return New(BadRequest, "The request body is not valid JSON: it ends before the value is complete.")
	case errors.As(err, &sizeErr):
		return New(BadRequest, fmt.Sprintf("The request body is larger than the %d byte limit.", sizeErr.Limit))
	case errors.As(err, &syntaxErr):
		return New(BadRequest, fmt.Sprintf("The request body is not valid JSON: %s (at byte %d).", strings.TrimPrefix(syntaxErr.Error(), "json: "), syntaxErr.Offset))
	case errors.As(err, &typeErr):
		return typeMismatch(typeErr)
	case errors.As(err, &timeErr):
		return New(BadRequest, fmt.Sprintf("The request body has a time that is not RFC 3339 (such as 2026-10-01T09:00:00Z): %q.", strings.Trim(timeErr.Value, `"`)))
	case errors.As(err, &textErr):
		return New(BadRequest, fmt.Sprintf("Field %q contains a NUL character, which is not supported.", textErr.field))
	case strings.HasPrefix(err.Error(), "invalid UUID"):
		return New(BadRequest, "The request body has a value that should be a UUID and is not one.")
	}
	if fes := fieldErrors(err); len(fes) > 0 {
		return fieldProblems(fes)
	}
	return New(BadRequest, "The request body could not be read in the shape this endpoint expects. Check it against the API reference.")
}

// InvalidQuery is the 400 for query parameters that failed to bind.
func InvalidQuery(err error) *Error {
	var textErr *unsupportedTextError
	if errors.As(err, &textErr) {
		return New(BadRequest, fmt.Sprintf("Query parameter %q contains a NUL character, which is not supported.", textErr.field))
	}
	return New(BadRequest, "The query parameters are invalid. Check them against the API reference.")
}

func typeMismatch(e *json.UnmarshalTypeError) *Error {
	want := jsonKind(e.Type)
	got, _, _ := strings.Cut(e.Value, " ")
	if e.Field == "" {
		return New(BadRequest, fmt.Sprintf("The request body must be a JSON %s, not a JSON %s.", want, got))
	}
	return New(BadRequest, fmt.Sprintf("Field %q must be a JSON %s, not a JSON %s.", e.Field, want, got))
}

// jsonKind names a Go type by the JSON value that decodes into it.
func jsonKind(t reflect.Type) string {
	if t == nil {
		return "value"
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Implements(textUnmarshaler) || reflect.PointerTo(t).Implements(textUnmarshaler) {
		return "string"
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Struct, reflect.Map:
		return "object"
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	}
	return "value"
}

var textUnmarshaler = reflect.TypeFor[interface{ UnmarshalText([]byte) error }]()

// fieldErrors flattens gin's per-element errors for a slice body into one list.
func fieldErrors(err error) []validator.FieldError {
	var out []validator.FieldError
	var sliceErrs binding.SliceValidationError
	var structErrs validator.ValidationErrors
	switch {
	case errors.As(err, &sliceErrs):
		for _, e := range sliceErrs {
			out = append(out, fieldErrors(e)...)
		}
	case errors.As(err, &structErrs):
		out = append(out, structErrs...)
	}
	return out
}

func fieldProblems(errs []validator.FieldError) *Error {
	parts := make([]string, 0, maxBindProblems)
	for i, fe := range errs {
		if i == maxBindProblems {
			parts = append(parts, fmt.Sprintf("and %d more", len(errs)-maxBindProblems))
			break
		}
		parts = append(parts, fieldProblem(fe))
	}
	return New(BadRequest, "The request body is missing or has invalid fields: "+strings.Join(parts, "; ")+".")
}

func fieldProblem(fe validator.FieldError) string {
	name := fieldPath(fe)
	unit := ""
	switch fe.Kind() {
	case reflect.String:
		unit = " characters"
	case reflect.Slice, reflect.Array, reflect.Map:
		unit = " items"
	}
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%q is required", name)
	case "min", "gte":
		return fmt.Sprintf("%q must be at least %s%s", name, fe.Param(), unit)
	case "max", "lte":
		return fmt.Sprintf("%q must be at most %s%s", name, fe.Param(), unit)
	case "gt":
		return fmt.Sprintf("%q must be more than %s%s", name, fe.Param(), unit)
	case "lt":
		return fmt.Sprintf("%q must be less than %s%s", name, fe.Param(), unit)
	case "len":
		return fmt.Sprintf("%q must be exactly %s%s", name, fe.Param(), unit)
	case "oneof":
		return fmt.Sprintf("%q must be one of: %s", name, strings.ReplaceAll(fe.Param(), " ", ", "))
	case "email":
		return fmt.Sprintf("%q must be an email address", name)
	case "uuid", "uuid4":
		return fmt.Sprintf("%q must be a UUID", name)
	case "url", "http_url":
		return fmt.Sprintf("%q must be a URL", name)
	}
	return fmt.Sprintf("%q failed the %q rule", name, fe.Tag())
}

// fieldPath is the dotted json path of a failing field, without the Go type the namespace starts with.
func fieldPath(fe validator.FieldError) string {
	ns := fe.Namespace()
	if _, rest, ok := strings.Cut(ns, "."); ok && rest != "" {
		return rest
	}
	return fe.Field()
}
