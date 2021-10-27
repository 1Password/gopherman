package postman_test

import (
	"strings"
	"testing"

	"github.com/1Password/gopherman/postman"
)

func TestSubstVars(t *testing.T) {
	type testCase struct {
		template    string
		values      []postman.Variable
		wantOut     string
		wantFailure bool
	}

	basicTemplate := `{\"value":\"{{ .Foo }}\"}`

	testCases := map[string]testCase{
		"no value": {
			template: basicTemplate,
			values:   []postman.Variable{},
			wantOut:  `{\"value":\"<no value>\"}`,
		},
		"emtpy disabled": {
			template: basicTemplate,
			values: []postman.Variable{
				{
					Key:     "Foo",
					Value:   "",
					Enabled: false,
				},
			},
			wantOut: `{\"value":\"<no value>\"}`,
		},
		"emtpy wrong key": {
			template: basicTemplate,
			values: []postman.Variable{
				{
					Key:     "Baz",
					Value:   "",
					Enabled: false,
				},
			},
			wantOut: `{\"value":\"<no value>\"}`,
		},
		"emtpy enabled": {
			template: basicTemplate,
			values: []postman.Variable{
				{
					Key:     "Foo",
					Value:   "",
					Enabled: true,
				},
			},
			wantOut: `{\"value":\"\"}`,
		},
		"basic substitution": {
			template: basicTemplate,
			values: []postman.Variable{
				{
					Key:     "Foo",
					Value:   "Bar",
					Enabled: true,
				},
			},
			wantOut: `{\"value":\"Bar\"}`,
		},
		"urlunsafe substitution": {
			template: basicTemplate,
			values: []postman.Variable{
				{
					Key:     "Foo",
					Value:   "foo+bar@example.com",
					Enabled: true,
				},
			},
			wantOut: `{\"value":\"foo+bar@example.com\"}`,
		},
		"urlsafe substitution": {
			template: basicTemplate,
			values: []postman.Variable{
				{
					Key:     "Foo",
					Value:   "foo%2Bbar%40example.com",
					Enabled: true,
				},
			},
			wantOut: `{\"value":\"foo%2Bbar%40example.com\"}`,
		},
		"html template substitution": {
			template: basicTemplate,
			values: []postman.Variable{
				{
					Key:     "Foo",
					Value:   "foo+bar@example.com",
					Enabled: true,
				},
			},
			wantOut:     `{\"value":\"foo&#43;bar@example.com\"}`,
			wantFailure: true,
		},
	}

	for desc, test := range testCases {
		t.Run(desc, func(t *testing.T) {
			env := postman.Environment{
				Values: test.values,
			}
			out, err := postman.SubstVars(basicTemplate, env.VariableMap())
			if err != nil {
				t.Fatalf("failed to SubstVars: %s", err.Error())
			}

			if !strings.EqualFold(out, test.wantOut) && !test.wantFailure {
				t.Errorf("got: %v; wanted: %v", out, test.wantOut)
			}
		})
	}

	return
}
