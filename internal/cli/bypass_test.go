package cli

import "testing"

func TestNormalizeBypass(t *testing.T) {
	cases := map[string]struct {
		in      string
		want    string
		wantErr bool
	}{
		"empty defaults to ru": {in: "", want: "ru"},
		"ru":                   {in: "ru", want: "ru"},
		"off":                  {in: "off", want: "off"},
		"uppercase RU":         {in: "RU", want: "ru"},
		"invalid":              {in: "us", wantErr: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := normalizeBypass(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("normalizeBypass(%q) = %q, want error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeBypass(%q) error: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("normalizeBypass(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
