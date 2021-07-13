package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/Masterminds/squirrel"
	"github.com/markcaudill/terraform-http-backend/terraform"

	_ "github.com/mattn/go-sqlite3"
)

/////////////////////////////
// Side Effects
/////////////////////////////
// openDB opens, and creates if necessary, an SQLite3 database using the
// provided path. Both the database file and schema are created.
func openDB(path string, schema *terraform.StateSchema) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Create the table if it doesn't exist
	_, err = db.Exec(schema.CreateStatement)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// httpResponse writes a response to an http.ResponseWriter and logs data to
// STDOUT
func httpResponse(w http.ResponseWriter, code int, body string) {
	log.Printf("Response: %d %s", code, body)
	w.WriteHeader(code)
	fmt.Fprint(w, body)
}

///////////////////
// End Side Effects
///////////////////

// GetStateID hashes the URL.Path, username, and password from the Request
// into a string suitable for use as a unique identifier
func GetStateID(req *http.Request, hasher hash.Hash) string {
	username, password, _ := req.BasicAuth()
	input := fmt.Sprintf("%s%s%s", req.URL.Path, username, password)
	hasher.Write([]byte(input))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// GetLockID parses JSON looking for an "ID" field (at the top level) and
// returns the value
func GetLockID(rawData []byte) (string, error) {
	var (
		id         string = ""
		parsedData map[string]string
		err        error
		ok         bool
	)
	err = json.Unmarshal(rawData, &parsedData)
	if err != nil {
		return "", err
	}
	id, ok = parsedData["ID"]
	if !ok {
		return "", fmt.Errorf("Error parsing \"ID\" from %s", rawData)
	}
	return id, nil
}

func stateHandler(db *sql.DB, schema *terraform.StateSchema) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		var (
			stateID      string
			currentState *terraform.State
			err          error
		)

		body, err := io.ReadAll(req.Body)
		defer req.Body.Close()
		if err != nil {
			httpResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		log.Printf("%s %s: %s", req.Method, req.URL.Path, body)

		stateID = GetStateID(req, sha256.New())
		currentState, err = schema.GetState(req.Context(), db, stateID)
		if err != nil {
			httpResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if currentState == nil {
			currentState = &terraform.State{nil, nil}
		}

		switch req.Method {
		case "LOCK":
			if string(currentState.Lock) == "" {
				currentState.Lock = body
				err = schema.SaveState(req.Context(), db, currentState, stateID)
				if err != nil {
					httpResponse(w, http.StatusInternalServerError, err.Error())
				} else {
					httpResponse(w, http.StatusOK, "")
				}
			} else {
				httpResponse(w, http.StatusLocked, string(currentState.Lock))
			}
			return
		case "UNLOCK":
			if string(currentState.Lock) == "" {
				httpResponse(w, http.StatusOK, "")
			} else {
				currentState.Lock = nil
				err := schema.SaveState(req.Context(), db, currentState, stateID)
				if err != nil {
					httpResponse(w, http.StatusInternalServerError, err.Error())
				} else {
					httpResponse(w, http.StatusOK, "")
				}
			}
			return
		case http.MethodGet:
			httpResponse(w, http.StatusOK, string(currentState.Data))
			return
		case http.MethodPost:
			// If there's an existing lock, verify this request is allowed to
			// be executed
			if string(currentState.Lock) != "" {
				reqLockID := req.FormValue("ID")
				currLockID, err := GetLockID(currentState.Lock)
				if err != nil {
					httpResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if reqLockID != currLockID {
					httpResponse(w, http.StatusLocked, string(currentState.Lock))
					return
				}
			}
			currentState.Data = body
			err := schema.SaveState(req.Context(), db, currentState, stateID)
			if err != nil {
				httpResponse(w, http.StatusInternalServerError, err.Error())
			} else {
				httpResponse(w, http.StatusOK, "")
			}
			return
		case http.MethodDelete:
			// If there's an existing lock, verify this request is allowed to
			// be executed
			if string(currentState.Lock) != "" {
				reqLockID := req.FormValue("ID")
				currLockID, err := GetLockID(currentState.Lock)
				if err != nil {
					httpResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if reqLockID != currLockID {
					httpResponse(w, http.StatusLocked, string(currentState.Lock))
					return
				}
			}
			currentState.Data = nil
			err := schema.SaveState(req.Context(), db, currentState, stateID)
			if err != nil {
				httpResponse(w, http.StatusInternalServerError, err.Error())
			} else {
				httpResponse(w, http.StatusOK, "")
			}
			return
		default:
			httpResponse(w, http.StatusNotImplemented, "Not implemented")
			return
		}
	}
}

func dbDumpHandler(db *sql.DB, schema *terraform.StateSchema) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		var states []terraform.State
		var err error
		body, err := io.ReadAll(req.Body)
		defer req.Body.Close()
		if err != nil {
			httpResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		log.Printf("%s %s %s", req.Method, req.URL.Path, body)
		rows, err := squirrel.
			Select(schema.DataColumnName, schema.LockColumnName).
			From(schema.TableName).
			RunWith(db).
			QueryContext(req.Context())
		if err != nil {
			httpResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		for rows.Next() {
			var state terraform.State
			err = rows.Scan(&state.Data, &state.Lock)
			if err != nil {
				break
			}
			states = append(states, state)
		}
		jsonStates, err := json.Marshal(states)
		if err != nil {
			httpResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpResponse(w, http.StatusOK, string(jsonStates))
	}
}

func main() {
	var (
		schema        *terraform.StateSchema
		dbFile        string
		envListenIP   string
		envListenPort string
		listenIP      net.IP
		listenPort    int
	)

	// Gather configuration from environment
	dbFile = os.Getenv("DATABASE")
	if dbFile == "" {
		dbFile = "state.db"
	}
	envListenIP = os.Getenv("IP")
	if envListenIP == "" {
		envListenIP = "127.0.0.1"
	}
	listenIP = net.ParseIP(envListenIP)
	if listenIP == nil {
		log.Fatalf("Unable to parse %s to net.IP", envListenIP)
	}
	envListenPort = os.Getenv("PORT")
	if envListenPort == "" {
		envListenPort = "8080"
	}
	parsedListenPort, err := strconv.Atoi(envListenPort)
	if err != nil {
		log.Fatalf("Unable to convert %s to int", envListenPort)
	}
	listenPort = int(parsedListenPort)

	// Construct configuration
	listenAddr := net.TCPAddr{IP: listenIP, Port: listenPort}
	schema = terraform.DefaultStateSchema()
	db, err := openDB(dbFile, schema)
	defer db.Close()
	if err != nil {
		log.Fatalf("openDB(%+v) = %v, %v", &schema, db, err)
	}

	// Start serving
	log.Printf("Listening on %s", listenAddr.String())
	http.HandleFunc("/s/", stateHandler(db, schema))
	http.HandleFunc("/health/dump/", dbDumpHandler(db, schema))
	log.Fatal(http.ListenAndServe(listenAddr.String(), nil))
}
