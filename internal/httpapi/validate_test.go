package httpapi

import "testing"

func TestValidK8sName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"tenant-a-dev", true},
		{"a", true},
		{"a1", true},
		{"123", true},
		{"", false},
		{"Tenant-A", false},               // uppercase not allowed
		{"tenant_a", false},               // underscore not allowed
		{"-tenant", false},                // can't start with '-'
		{"tenant-", false},                // can't end with '-'
		{"tenant/a", false},               // path separator
		{"../secrets", false},             // traversal
		{"..%2fsecrets", false},           // encoded traversal attempt (literal percent chars aren't valid either)
		{"tenant a", false},               // space
		{"tenant\x00a", false},            // embedded NUL
		{string(make([]byte, 64)), false}, // too long (64 NUL bytes, also fails the charset check)
	}
	for _, c := range cases {
		if got := validK8sName(c.name); got != c.want {
			t.Errorf("validK8sName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestValidK8sName_RejectsNameLongerThan63Characters(t *testing.T) {
	// 64 lowercase letters - otherwise charset-valid, only too long.
	name := ""
	for i := 0; i < 64; i++ {
		name += "a"
	}
	if validK8sName(name) {
		t.Errorf("validK8sName(64-char name) = true, want false")
	}
}

func TestValidK8sName_Accepts63Characters(t *testing.T) {
	name := "a"
	for i := 0; i < 61; i++ {
		name += "a"
	}
	name += "a" // exactly 63 'a's
	if len(name) != 63 {
		t.Fatalf("test setup bug: len(name) = %d, want 63", len(name))
	}
	if !validK8sName(name) {
		t.Errorf("validK8sName(63-char name) = false, want true")
	}
}
