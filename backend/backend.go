package backend

import (
	"fmt"
	"time"
)

type Snapshot struct {
	N     int
	Name  string
	Count DiffResultCount
	Time  time.Time
}

type SnapshotVars struct {
	Message string
	Vars    map[string]string
}

type SourceBackend interface {
	TakeSnapshot(message string) (SnapshotVars, error)
	Close()
}

type Backend interface {
	ListSnapshots() ([]Snapshot, error)
	LoadSnapshot(filename string) (SnapshotVars, error)
	SaveSnapshot(vars SnapshotVars) error
	Close()
}

type DiffResultCount struct {
	Added, Changed, Removed int
}

type DiffResult struct {
	Lines    []DiffLine
	Count    DiffResultCount
	From, To int
}

type DiffLine struct {
	Path, OldValue, NewValue      string
	IsAdded, IsRemoved, IsChanged bool
	Scope                         string
	ScopeId                       int64
}

func (this *Snapshot) String() string {
	return fmt.Sprintf("% 4d %-20s %s %v", this.N, this.Name, this.Count.String(), this.Time)
}

func (self *DiffResultCount) Changes() int {
	return self.Added + self.Removed + self.Changed
}

func (self *DiffResultCount) String() string {
	return fmt.Sprintf("A%d C%d R%d",
		self.Added, self.Removed, self.Changed)
}
