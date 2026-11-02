#!/usr/bin/env bash
# Creates the small Go project that the screenshots are taken in.
# usage: demo.sh DIR (DIR is removed and created again)
set -euo pipefail

dir=$1
here=$(cd "$(dirname "$0")" && pwd)
rm -rf "$dir"
mkdir -p "$dir/internal/store" "$dir/docs"
cp "$here/../../manual/docs/assets/icon.png" "$dir/docs/icon.png"
cd "$dir"

cat >go.mod <<'EOF'
module example.com/todo

go 1.22
EOF

cat >main.go <<'EOF'
package main

import (
	"fmt"
	"os"

	"example.com/todo/internal/store"
)

func main() {
	s := store.New()
	s.Add("Write the README")
	s.Add("Fix the flaky test")
	s.Done(0)

	for i, t := range s.List() {
		mark := " "
		if t.Done {
			mark = "x"
		}
		fmt.Fprintf(os.Stdout, "%d. [%s] %s\n", i+1, mark, t.Title)
	}
}
EOF

cat >internal/store/store.go <<'EOF'
// Package store keeps the to-do items in memory.
package store

// Task is one to-do item.
type Task struct {
	Title string
	Done  bool
}

// Store holds the tasks in the order they were added.
type Store struct {
	tasks []Task
}

// New returns an empty store.
func New() *Store {
	return &Store{}
}

// Add appends a task that is not done yet.
func (s *Store) Add(title string) {
	s.tasks = append(s.tasks, Task{Title: title})
}

// Done marks the task at index i as done.
func (s *Store) Done(i int) {
	s.tasks[i].Done = true
}

// List returns all tasks.
func (s *Store) List() []Task {
	return s.tasks
}
EOF

cat >README.md <<'EOF'
# todo

A tiny to-do list used to try out **shijimi-editor**.

## Commands

| Command | What it does |
| --- | --- |
| `go run .` | Print the list |
| `go test ./...` | Run the tests |

## Roadmap

- [x] Add tasks
- [x] Mark tasks as done
- [ ] Remove tasks
- [ ] Save the list to a file

```go
s := store.New()
s.Add("Write the README")
```

> Keep it small. This project exists only for the screenshots.
EOF

cat >docs/design.md <<'EOF'
# Design

The program keeps the tasks in memory and prints them in order.

```mermaid
flowchart LR
    main[main.go] -->|Add / Done| store[(store.Store)]
    store -->|List| main
```

- `main.go` adds the tasks and prints the list
- `store.Store` keeps the tasks in the order they were added

EOF

git init -q -b main
git add -A
git -c user.name=demo -c user.email=demo@example.com commit -q -m init

# An uncommitted change for diff mode.
cat >internal/store/store.go <<'EOF'
// Package store keeps the to-do items in memory.
package store

// Task is one to-do item.
type Task struct {
	Title string
	Done  bool
}

// Store holds the tasks in the order they were added.
type Store struct {
	tasks []Task
}

// New returns an empty store.
func New() *Store {
	return &Store{}
}

// Add appends a task that is not done yet.
func (s *Store) Add(title string) {
	s.tasks = append(s.tasks, Task{Title: title})
}

// Done marks the task at index i as done.
// It reports false when i is out of range.
func (s *Store) Done(i int) bool {
	if i < 0 || i >= len(s.tasks) {
		return false
	}
	s.tasks[i].Done = true
	return true
}

// Remove deletes the task at index i.
func (s *Store) Remove(i int) {
	s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
}

// List returns all tasks, done or not.
func (s *Store) List() []Task {
	return s.tasks
}
EOF
