package model

import "testing"

func TestSQLiteDSN(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "memory passthrough",
			path: ":memory:",
			want: ":memory:",
		},
		{
			name: "plain path gets query string",
			path: "/data/jieli.db",
			want: "/data/jieli.db?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_txlock=immediate",
		},
		{
			name: "path with existing query appends with ampersand",
			path: "/data/jieli.db?cache=shared",
			want: "/data/jieli.db?cache=shared&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_txlock=immediate",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sqliteDSN(tc.path); got != tc.want {
				t.Fatalf("sqliteDSN(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
