package main

import (
	"crypto/sha256"
	"hash"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
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
	var (
		testInput      = `{"ID":"ebd786a4-fb72-f8ad-9705-d6ba623552c2","Operation":"OperationTypeApply","Info":"","Who":"mark@fry","Version":"1.0.1","Created":"2021-07-12T17:29:27.616435429Z","Path":""}`
		expectedOutput = `ebd786a4-fb72-f8ad-9705-d6ba623552c2`
	)
	actualOutput, err := GetLockID([]byte(testInput))
	if err != nil {
		t.Fatalf("Error %v", err)
	}
	if actualOutput != expectedOutput {
		t.Fatalf("Got %s, expected %s", actualOutput, expectedOutput)
	}
}
