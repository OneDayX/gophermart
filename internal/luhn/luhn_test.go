package luhn

import "testing"

func TestValid(t *testing.T) {
	tests := []struct {
		number string
		want   bool
	}{
		{number: "12345678903", want: true},
		{number: "2377225624", want: true},
		{number: "12345678901", want: false},
		{number: "1234a678903", want: false},
		{number: "", want: false},
	}

	for _, tt := range tests {
		if got := Valid(tt.number); got != tt.want {
			t.Errorf("Valid(%q) = %v, want %v", tt.number, got, tt.want)
		}
	}
}
