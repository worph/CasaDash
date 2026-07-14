package config

import "testing"

func TestIsProtected(t *testing.T) {
	c := Config{ProtectedApps: []string{"casadash", "CasaOS"}}

	cases := []struct {
		name    string
		storeID string
		project string
		want    bool
	}{
		{"store id matches", "casadash", "my-dashboard", true},
		{"project name matches when no store id", "", "casadash", true},
		{"match is case-insensitive", "casaos", "whatever", true},
		{"unrelated app", "nextcloud", "nextcloud", false},
		{"empty identifiers never match", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.IsProtected(tc.storeID, tc.project); got != tc.want {
				t.Fatalf("IsProtected(%q, %q) = %v, want %v", tc.storeID, tc.project, got, tc.want)
			}
		})
	}

	if (Config{}).IsProtected("casadash", "casadash") {
		t.Fatal("nothing is protected when PROTECTED_APPS is unset")
	}
}
