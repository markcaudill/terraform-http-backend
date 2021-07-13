package terraform

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Masterminds/squirrel"
)

type StateSchema struct {
	TableName       string
	IDColumnName    string
	DataColumnName  string
	LockColumnName  string
	CreateStatement string
}

func DefaultStateSchema() *StateSchema {
	return &StateSchema{
		TableName:      "state",
		IDColumnName:   "id",
		DataColumnName: "data",
		LockColumnName: "lock",
		CreateStatement: `
		CREATE TABLE IF NOT EXISTS state (
			id TEXT PRIMARY KEY,
			data BLOB,
			lock BLOB
		)`,
	}
}

// UpsertState generates a squirrel.InsertBuilder suitable for inserting
// a new row or updating if the stateID already exists
func (c *StateSchema) UpsertState(state *State, stateID string) squirrel.InsertBuilder {
	return squirrel.
		Insert(c.TableName).
		Columns(c.IDColumnName, c.DataColumnName, c.LockColumnName).
		Values(stateID, string(state.Data), string(state.Lock)).
		Suffix(
			fmt.Sprintf("ON CONFLICT(%s) DO UPDATE SET %s = ?, %s = ?",
				c.IDColumnName, c.DataColumnName, c.LockColumnName),
			string(state.Data), string(state.Lock))
}

// SelectState generates a squirrel.SelectBuilder suitable for selecting
// a row
func (c *StateSchema) SelectState(stateID string) squirrel.SelectBuilder {
	return squirrel.
		Select(c.DataColumnName, c.LockColumnName).
		From(c.TableName).
		Where(squirrel.Eq{c.IDColumnName: stateID})
}

///////////////
// Side Effects
///////////////

// SaveState saves a TerraformState struct to a database
func (c *StateSchema) SaveState(ctx context.Context, db *sql.DB, state *State, stateID string) error {
	_, err := c.UpsertState(state, stateID).RunWith(db).ExecContext(ctx)
	return err
}

// GetState gets a TerraformState struct from the database
// if no rows are found then a new row is created
func (c *StateSchema) GetState(ctx context.Context, db *sql.DB, stateID string) (*State, error) {
	var (
		state State
		err   error
	)

	err = c.SelectState(stateID).RunWith(db).QueryRowContext(ctx).Scan(&state.Data, &state.Lock)
	// We don't care about this error
	if err == sql.ErrNoRows {
		err = nil
	}

	return &state, nil
}

///////////////////
// End Side Effects
///////////////////
