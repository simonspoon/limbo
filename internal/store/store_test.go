package store

import (
	"reflect"
	"testing"

	"github.com/simonspoon/limbo/internal/models"
)

// stubStore is a no-op implementation used only to assert that the Store
// interface's method set is satisfiable and stable. It is not a functional
// backend — the JSON backend lands in a later task.
type stubStore struct{}

func (stubStore) Load() ([]models.Task, error)         { return nil, nil }
func (stubStore) SaveAll(tasks []models.Task) error    { return nil }
func (stubStore) Revision() (int, error)               { return 0, nil }
func (stubStore) AddTask(task models.Task) error       { return nil }
func (stubStore) UpdateTask(task models.Task) error    { return nil }
func (stubStore) DeleteTask(id string) error           { return nil }
func (stubStore) AppendNote(id, content string) error  { return nil }
func (stubStore) ReadContext(taskID string) (string, error) {
	return "", nil
}
func (stubStore) WriteContext(taskID, content string) error { return nil }

// Compile-time assertion that stubStore satisfies Store.
var _ Store = stubStore{}

// TestStoreInterfaceMethodSet pins the required method set so an accidental
// removal of a method (or a signature change) fails loudly here as well as at
// compile time.
func TestStoreInterfaceMethodSet(t *testing.T) {
	required := []string{
		"Load", "SaveAll", "Revision",
		"AddTask", "UpdateTask", "DeleteTask", "AppendNote",
		"ReadContext", "WriteContext",
	}
	typ := reflect.TypeOf((*Store)(nil)).Elem()
	for _, name := range required {
		if _, ok := typ.MethodByName(name); !ok {
			t.Errorf("Store interface missing required method %q", name)
		}
	}
}
