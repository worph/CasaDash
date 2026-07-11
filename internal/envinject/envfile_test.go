package envinject

import (
	"reflect"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	in := []byte("# seeded at install\nPUID=1000\n\nTZ = Europe/Paris\nbogus-line\nPUID=1001\n")
	got := ParseEnvFile(in)
	want := []Var{
		{Key: "PUID", Value: "1001"}, // duplicate: first position, last value — as compose resolves it
		{Key: "TZ", Value: "Europe/Paris"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEnvFile() = %#v, want %#v", got, want)
	}
}

// The whole point of patching rather than regenerating: an operator's comments,
// blank lines and key order survive a save from the editor.
func TestPatchEnvFilePreservesCommentsAndOrder(t *testing.T) {
	old := []byte("# CasaDash seeded these\nPUID=1000\nPGID=1000\n\n# my own note\nTZ=UTC\n")
	got, err := PatchEnvFile(old, []Var{
		{Key: "PUID", Value: "1000"},
		{Key: "TZ", Value: "Europe/Paris"}, // edited
		{Key: "API_KEY", Value: "s3cret"},  // added
		// PGID removed
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "# CasaDash seeded these\nPUID=1000\n\n# my own note\nTZ=Europe/Paris\nAPI_KEY=s3cret\n"
	if string(got) != want {
		t.Fatalf("PatchEnvFile() =\n%q\nwant\n%q", got, want)
	}
}

func TestPatchEnvFileEmptiedFile(t *testing.T) {
	got, err := PatchEnvFile([]byte("PUID=1000\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("PatchEnvFile(all removed) = %q, want nil", got)
	}
}

func TestPatchEnvFileRejectsBadVars(t *testing.T) {
	cases := map[string][]Var{
		"empty key":       {{Key: "", Value: "x"}},
		"leading digit":   {{Key: "2FA", Value: "x"}},
		"shell-unsafe":    {{Key: "MY-KEY", Value: "x"}},
		"duplicate":       {{Key: "A", Value: "1"}, {Key: "A", Value: "2"}},
		"multiline value": {{Key: "A", Value: "one\ntwo"}},
	}
	for name, vars := range cases {
		if _, err := PatchEnvFile([]byte("A=0\n"), vars); err == nil {
			t.Errorf("PatchEnvFile(%s) succeeded, want error", name)
		}
	}
}

// A value may legitimately contain '=', spaces or a '#' — .env has no escaping,
// so everything after the first '=' is the value and must round-trip verbatim.
func TestPatchEnvFileValueRoundTrip(t *testing.T) {
	vars := []Var{{Key: "DSN", Value: "user:p@ss=w0rd@host/db?x=1 #notacomment"}}
	got, err := PatchEnvFile(nil, vars)
	if err != nil {
		t.Fatal(err)
	}
	if back := ParseEnvFile(got); !reflect.DeepEqual(back, vars) {
		t.Fatalf("round-trip = %#v, want %#v", back, vars)
	}
}
