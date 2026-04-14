package config

import "testing"

func TestParseUsers(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    map[string]string
		wantErr bool
	}{
		{"empty", "", map[string]string{}, false},
		{"single", "alice:$2a$10$xyz", map[string]string{"alice": "$2a$10$xyz"}, false},
		{"two", "alice:h1,bob:h2", map[string]string{"alice": "h1", "bob": "h2"}, false},
		{"bcrypt with colons", "alice:$2a$10$abc:def", map[string]string{"alice": "$2a$10$abc:def"}, false},
		{"missing hash", "alice:", nil, true},
		{"missing user", ":hash", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseUsers(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("got[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
