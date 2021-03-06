package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"hash"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
			`{"ID":"ebd786a4-fb72-f8ad-9705-d6ba623552c2",
			  "Operation":"OperationTypeApply",
			  "Info":"",
			  "Who":"mark@fry",
			  "Version":"1.0.1",
			  "Created":"2021-07-12T17:29:27.616435429Z",
			  "Path":""}`,
			func(str string) {
				if str != `ebd786a4-fb72-f8ad-9705-d6ba623552c2` {
					t.Errorf("got %v, expected %s",
						str, `ebd787a4-fb72-f8ad-9705-d6ba623552c2`)
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
			`{"ID:"ebd786a4-fb72-f8ad-9705-d6ba623552c2",
			  "Operation":"OperationTypeApply",
			  "Info":"",
			  "Who":"mark@fry",
			  "Version":"1.0.1",
			  "Created":"2021-07-12T17:29:27.616435429Z",
			  "Path":""}`,
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
			`{"Operation":"OperationTypeApply",
			  "Info":"",
			  "Who":"mark@fry",
			  "Version":"1.0.1",
			  "Created":"2021-07-12T17:29:27.616435429Z",
			  "Path":""}`,
			func(str string) {
				if str != `` {
					t.Errorf("got %v, expected %s", str, ``)
				}
			},
			func(err error) {
				if !strings.Contains(err.Error(), `error parsing "ID" from`) {
					t.Errorf(`got %v, expected "error parsing "ID" from"`, err)
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

func TestHTTPHandlers(t *testing.T) {
	tt := []struct {
		name                       string
		dbPath                     string
		dbSetup                    func(*sql.DB) error
		schema                     *terraform.StateSchema
		requestMethod              string
		requestPath                string
		requestBody                []byte
		expectedResponseStatusCode int
		expectedResponseBody       []byte
	}{
		{
			"GET with empty state database",
			":memory:",
			func(db *sql.DB) error { return nil },
			terraform.DefaultStateSchema(),
			http.MethodGet,
			"/",
			[]byte(""),
			http.StatusOK,
			[]byte(""),
		},
		{
			"LOCK with empty state database",
			":memory:",
			func(db *sql.DB) error { return nil },
			terraform.DefaultStateSchema(),
			"LOCK",
			"/",
			[]byte(`{"ID":"testlockid"}`),
			200,
			[]byte(""),
		},
		{
			"LOCK with conflicting lock in state database",
			"test.db",
			func(db *sql.DB) error {
				_, err := terraform.DefaultStateSchema().
					UpsertState(
						&terraform.State{
							Data: nil,
							Lock: []byte(`{"ID":"testlockid"}`)},
						GetStateID(
							&http.Request{
								URL: &url.URL{
									Path: "/",
								},
							},
							sha256.New())).
					RunWith(db).Exec()
				return err
			},
			terraform.DefaultStateSchema(),
			"LOCK",
			"/",
			[]byte(`{"ID":"testlockid2"}`),
			http.StatusLocked,
			[]byte(`{"ID":"testlockid"}`),
		},
		{
			"UNLOCK with empty state database",
			":memory:",
			func(db *sql.DB) error { return nil },
			terraform.DefaultStateSchema(),
			"UNLOCK",
			"/?ID=testlockid",
			[]byte(""),
			200,
			[]byte(""),
		},
		{
			"UNLOCK with correct/matching lock in state database",
			"test.db",
			func(db *sql.DB) error {
				_, err := terraform.DefaultStateSchema().
					UpsertState(
						&terraform.State{
							Data: nil,
							Lock: []byte(`{"ID":"testlockid"}`)},
						GetStateID(
							&http.Request{
								URL: &url.URL{
									Path: "/",
								},
							},
							sha256.New())).
					RunWith(db).Exec()
				return err
			},
			terraform.DefaultStateSchema(),
			"UNLOCK",
			"/?ID=testlockid",
			[]byte(""),
			http.StatusOK,
			[]byte(""),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			db, err := openDB(tc.dbPath, tc.schema)
			if err != nil {
				t.Fatalf("openDB(): got %v, expected nil", err)
			}
			defer db.Close()
			err = tc.dbSetup(db)
			if err != nil {
				t.Fatalf("dbSetup(): got %v, expected nil", err)
			}
			req := httptest.NewRequest(tc.requestMethod, tc.requestPath, bytes.NewReader(tc.requestBody))
			w := httptest.NewRecorder()
			stateHandler(db, tc.schema)(w, req)
			res := w.Result()
			body, _ := io.ReadAll(res.Body)
			if res.StatusCode != tc.expectedResponseStatusCode {
				t.Errorf("got %d, expected %d", res.StatusCode, tc.expectedResponseStatusCode)
			}
			if string(body) != string(tc.expectedResponseBody) {
				t.Errorf("got %s, expected %s", body, tc.expectedResponseBody)
			}
		})
	}
}
