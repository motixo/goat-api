package valueobject

import "testing"

func TestParsePermission(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Permission
		wantErr bool
	}{
		{name: "known permission", input: "user:read", want: PermUserRead},
		{name: "full access", input: "full_access", want: PermFullAccess},
		{name: "unknown permission", input: "database:drop", wantErr: true},
		{name: "empty permission", input: "", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParsePermission(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("ParsePermission(%q) error = %v, wantErr %v", test.input, err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("ParsePermission(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
