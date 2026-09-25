package models

import "testing"

func TestParameterFieldEnvName(t *testing.T) {
	tests := []struct {
		name  string
		field ParameterField
		want  string
	}{
		{
			name:  "Success - an explicit envVar is used as written",
			field: ParameterField{Name: "environment", EnvVar: "TARGET_ENV"},
			want:  "TARGET_ENV",
		},
		{
			name:  "Success - a missing envVar is derived from the parameter name",
			field: ParameterField{Name: "dry_run"},
			want:  "DRY_RUN",
		},
		{
			name:  "Success - punctuation and spaces become underscores",
			field: ParameterField{Name: "release date/time"},
			want:  "RELEASE_DATE_TIME",
		},
		{
			name:  "Success - a leading digit is prefixed, since a variable cannot start with one",
			field: ParameterField{Name: "2fa"},
			want:  "P_2FA",
		},
		{
			name:  "Success - a name with nothing usable in it yields nothing",
			field: ParameterField{Name: "***"},
			want:  "",
		},
		{
			name:  "Success - an empty name yields nothing",
			field: ParameterField{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.field.EnvName(); got != tt.want {
				t.Errorf("EnvName() = %q, want %q", got, tt.want)
			}
		})
	}
}
