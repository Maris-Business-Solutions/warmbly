package validate

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestUuids(t *testing.T) {
	id := uuid.NewString()
	for _, tc := range []struct {
		name     string
		in, want []string
		invalid  bool
	}{
		{"omitted", nil, nil, false},
		{"clear", []string{}, []string{}, false},
		{"canonical set", []string{id, strings.ToUpper(id), strings.ReplaceAll(id, "-", "")}, []string{id}, false},
		{"invalid", []string{"cro-c6"}, nil, true},
		{"mixed", []string{id, "cro-c6"}, nil, true},
		{"empty id", []string{""}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Uuids(tc.in)
			if (err != nil) != tc.invalid || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Uuids = %v, %v; want %v, invalid=%t", got, err, tc.want, tc.invalid)
			}
		})
	}
}
