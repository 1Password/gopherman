package gopherman

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"

	"github.com/1Password/gopherman/postman"

	"github.com/stretchr/testify/assert"
)

// Tester represents a collection test tool
type Tester struct {
	Environment *postman.Environment
	Client      *http.Client
	Collections []postman.Collection
	Hostname    string
	Port        string
}

// NewTesterWithCollection loads a collection from a file
func NewTesterWithCollection(path string, envFile string, files ...string) (*Tester, error) {
	env, err := postman.EnvironmentFromFile(filepath.Join(path, envFile))
	if err != nil {
		return nil, err
	}

	collections := make([]postman.Collection, len(files))

	for i, name := range files {
		file, err := ioutil.ReadFile(filepath.Join(path, name))
		if err != nil {
			return nil, err
		}

		collection := postman.Collection{}
		if err := json.Unmarshal(file, &collection); err != nil {
			return nil, err
		}

		collections[i] = collection
	}

	tester := Tester{
		Environment: env,
		Client:      http.DefaultClient,
		Collections: collections,
		Hostname:    "localhost",
		Port:        "3002",
	}

	return &tester, nil
}

// TestRequestWithName finds the named request in the collection, makes the same request, and then returns the request, expected response, and actual response
func (t *Tester) TestRequestWithName(name string, tst *testing.T, handler func(*TestHelper, *postman.Request, *postman.Response, *postman.Response)) []error {
	vars := t.Environment.VariableMap()

	tmplHost, err := postman.SubstVars("{{ .BaseUrl }}:{{ .Port }}", vars)
	if err != nil {
		fmt.Println(err)
		tmplHost = "localhost:8080"
	}

	errs := []error{}

	for _, collection := range t.Collections {
		helper := NewTestHelper(tst)

		// put this in a func so that critical errors can be collected and then bail out
		func() {
			itm := collection.ItemWithName(name)
			if itm == nil {
				helper.Error(fmt.Errorf("item with name %s doesn't exist", name))
				return
			}

			httpReq := itm.Request.ToHTTPRequest(vars)
			if httpReq == nil {
				helper.Error(errors.New("failed to build HTTP request"))
				return
			}

			httpReq.URL.Host = tmplHost
			httpReq.URL.Scheme = "http"

			actual, err := makeRequest(t.Client, httpReq)
			if err != nil {
				helper.Error(err)
				return
			}

			switch {
			case &itm.Response == nil:
				handler(helper, &itm.Request, nil, actual)
			case len(itm.Response) == 0:
				handler(helper, &itm.Request, nil, actual)
			default:
				expected, err := (&itm.Response[0]).InflateEnvironmentVariables(vars)
				if err != nil {
					helper.Error(err)
					return
				}
				handler(helper, &itm.Request, expected, actual)
			}

		}()

		if helper.HasErrors() {
			errs = append(errs, helper.AnnotateErrors(collection.Info.Name, name)...)
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// AugmentEnvironment of the Tester with one more key value pair. These are set to text with no description and as enabled.
func (t *Tester) AugmentEnvironment(values map[string]string) {
	for k, v := range values {
		newValue := postman.Variable{
			Key:         k,
			Value:       v,
			Type:        "text",
			Description: "",
			Enabled:     true,
		}
		t.Environment.Values = append(t.Environment.Values, newValue)
	}

	return
}

func makeRequest(client *http.Client, req *http.Request) (*postman.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	actual := &postman.Response{
		Mode:   "raw",
		Raw:    string(body),
		Status: resp.StatusCode,
	}

	return actual, nil
}

////////// helper functions //////////

// AssertErrors loops through the collected errors and t.Error's each of them
func AssertErrors(t *testing.T, errs []error) {
	for _, e := range errs {
		t.Error(e)
	}
}

///////// TestHelper //////////////

// TestHelper helps with running tests
type TestHelper struct {
	Assert *assert.Assertions
	t      *testing.T
	errors []error
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T) *TestHelper {
	helper := &TestHelper{
		Assert: assert.New(t),
		t:      t,
		errors: []error{},
	}

	return helper
}

// HasErrors returns true if the testhelper has errors
func (t *TestHelper) HasErrors() bool {
	return len(t.errors) > 0
}

func (t *TestHelper) Error(err error) {
	t.errors = append(t.errors, err)
}

// Log logs something
func (t *TestHelper) Log(msg string) {
	t.t.Log(msg)
}

// AnnotateErrors adds collection and test names to errors held by t
func (t *TestHelper) AnnotateErrors(collectionName, testName string) []error {
	errs := []error{}

	for _, e := range t.errors {
		wrapped := errors.Wrapf(e, "(collection %s, request %s)", collectionName, testName)
		errs = append(errs, wrapped)
	}

	return errs
}
