package birthday

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		input string
		want  Date
		ok    bool
	}{
		{input: "01/01", want: Date{Month: time.January, Day: 1}, ok: true},
		{input: "29/02", want: Date{Month: time.February, Day: 29}, ok: true},
		{input: "29/02/2024", ok: false},
		{input: "31/04", ok: false},
		{input: "abc", ok: false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := ParseDate(test.input)
			if (err == nil) != test.ok {
				t.Fatalf("ParseDate(%q) error = %v, want success = %t", test.input, err, test.ok)
			}
			if test.ok && got != test.want {
				t.Fatalf("ParseDate(%q) = %#v, want %#v", test.input, got, test.want)
			}
		})
	}
}
