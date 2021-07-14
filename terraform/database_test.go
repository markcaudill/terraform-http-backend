package terraform

import (
	"fmt"
	"testing"
)

func TestUpsertState(t *testing.T) {
	var (
		state *State = &State{
			[]byte("{\"test\":\"data\"}"),
			[]byte("{\"test\":\"lock\"}")}
		schema         *StateSchema = DefaultStateSchema()
		stateID        string       = "a5a5a5a5"
		expectedOutput string       = fmt.Sprintf(
			"INSERT INTO %s (%s,%s,%s) VALUES (?,?,?) ON CONFLICT(%s) DO UPDATE SET %s = ?, %s = ?",
			schema.TableName, schema.IDColumnName, schema.DataColumnName, schema.LockColumnName,
			schema.IDColumnName, schema.DataColumnName, schema.LockColumnName)
		actualOutput string
		err          error
	)

	actualOutput, _, err = schema.UpsertState(state, stateID).ToSql()
	if err != nil {
		t.Fatalf("Error from UpsertState(): %s", err.Error())
	}
	if actualOutput != expectedOutput {
		t.Fatalf("%s != %s", actualOutput, expectedOutput)
	}
}

func TestSelectState(t *testing.T) {
	var (
		schema         *StateSchema = DefaultStateSchema()
		stateID        string       = "a5a5a5a5"
		expectedOutput string       = fmt.Sprintf(
			"SELECT %s, %s FROM %s WHERE %s = ?",
			schema.DataColumnName, schema.LockColumnName, schema.TableName,
			schema.IDColumnName)
		actualOutput string
		err          error
	)

	actualOutput, _, err = schema.SelectState(stateID).ToSql()
	if err != nil {
		t.Fatalf("Error from SelectState(): %s", err.Error())
	}
	if actualOutput != expectedOutput {
		t.Fatalf("%s != %s", actualOutput, expectedOutput)
	}
}
