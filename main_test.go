package main

import (
	"crypto/sha256"
	"database/sql"
	"hash"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/markcaudill/terraform-http-backend/terraform"
)

func TestGetStateID(t *testing.T) {
	var (
		testRequest    *http.Request
		hasher         hash.Hash
		expectedOutput string = "8a5edab282632443219e051e4ade2d1d5bbc671c781051bf1437897cbdfea0f1"
		actualOutput   string
		err            error
	)
	hasher = sha256.New()
	testRequest, err = http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	actualOutput = GetStateID(testRequest, hasher)
	if actualOutput != expectedOutput {
		t.Fatalf("%s != %s", actualOutput, expectedOutput)
	}
}

func TestHttpRepsonse(t *testing.T) {
	var (
		w    *httptest.ResponseRecorder = httptest.NewRecorder()
		code int                        = 200
		body string                     = "test response"
	)
	httpResponse(w, code, body)
	res := w.Result()
	if res.StatusCode != code {
		t.Errorf("res.StatusCode = %d, expected %d", res.StatusCode, code)
	}
	resBody, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("unable to read w.Body: %v", err)
	}
	if string(resBody) != body {
		t.Errorf("res.Body = %s, expected %s", resBody, body)
	}
}

func TestGetLockID(t *testing.T) {
	tt := []struct {
		name            string
		input           string
		outputValidator func(string)
		errValidator    func(error)
	}{
		{
			"good input",
			`{"ID":"ebd786a4-fb72-f8ad-9705-d6ba623552c2","Operation":"OperationTypeApply","Info":"","Who":"mark@fry","Version":"1.0.1","Created":"2021-07-12T17:29:27.616435429Z","Path":""}`,
			func(str string) {
				if str != `ebd786a4-fb72-f8ad-9705-d6ba623552c2` {
					t.Errorf("got %v, expected %s", str, `ebd786a4-fb72-f8ad-9705-d6ba623552c2`)
				}
			},
			func(err error) {
				if err != nil {
					t.Errorf("got %v, expected nil", err)
				}
			},
		},
		{
			"bad json",
			`{"ID:"ebd786a4-fb72-f8ad-9705-d6ba623552c2","Operation":"OperationTypeApply","Info":"","Who":"mark@fry","Version":"1.0.1","Created":"2021-07-12T17:29:27.616435429Z","Path":""}`,
			func(str string) {
				if str != `` {
					t.Errorf("got %v, expected %s", str, ``)
				}
			},
			func(err error) {
				if !strings.Contains(err.Error(), `invalid character`) {
					t.Errorf(`got %v, expected "invalid character ..."`, err)
				}
			},
		},
		{
			"no ID",
			`{"Operation":"OperationTypeApply","Info":"","Who":"mark@fry","Version":"1.0.1","Created":"2021-07-12T17:29:27.616435429Z","Path":""}`,
			func(str string) {
				if str != `` {
					t.Errorf("got %v, expected %s", str, ``)
				}
			},
			func(err error) {
				if !strings.Contains(err.Error(), `Error parsing "ID" from`) {
					t.Errorf(`got %v, expected "Error parsing "ID" from"`, err)
				}
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput, err := GetLockID([]byte(tc.input))
			tc.errValidator(err)
			tc.outputValidator(actualOutput)
		})
	}
}

func TestOpenDB(t *testing.T) {
	tt := []struct {
		name         string
		path         string
		schema       *terraform.StateSchema
		dbValidator  func(*sql.DB)
		errValidator func(error)
		cleanup      func()
	}{
		{"good file path", "test.db",
			terraform.DefaultStateSchema(),
			func(db *sql.DB) {
				if db == nil {
					t.Error("got nil, expected !nil")
				}
			},
			func(err error) {
				if err != nil {
					t.Errorf("got %v, expected nil", err)
				}
			},
			func() {
				os.Remove("test.db")
			},
		},
		{"generate error", "test.db?_txlock=bogus",
			terraform.DefaultStateSchema(),
			func(db *sql.DB) {
				if db != nil {
					t.Errorf("got %v, expected nil", db)
				}
			},
			func(err error) {
				if err == nil {
					t.Error("got nil, expected !nil")
				}
			},
			func() {},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.cleanup()
			db, err := openDB(tc.path, tc.schema)
			tc.errValidator(err)
			tc.dbValidator(db)
		})
	}
}
