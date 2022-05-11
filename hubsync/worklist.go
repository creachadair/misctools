package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/creachadair/atomicfile"
)

type workList struct {
	Base     string        `json:"base"`
	Original string        `json:"original"`
	Remote   string        `json:"remote"`
	Branches []*branchInfo `json:"branches"`
	Loaded   bool          `json:"-"`
}

func (w *workList) saveTo(path string) error {
	f, err := atomicfile.New(path, 0600)
	if err != nil {
		return err
	}
	defer f.Cancel()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(w); err != nil {
		return err
	}
	return f.Close()
}

func (w *workList) loadFrom(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, w)
}

func (w *workList) resetDir() {
	_, err := git("checkout", w.Original)
	if err != nil {
		log.Fatalf("Switching to %q: %v", w.Original, err)
	}
	log.Printf("Switched back to %q", w.Original)
}

func (w *workList) numUnfinished() int {
	var n int
	for _, b := range w.Branches {
		if !b.Done {
			n++
		}
	}
	return n
}

func openWorkList(path string) (*workList, error) {
	if path != "" {
		var work workList
		err := work.loadFrom(path)
		if err == nil {
			work.Loaded = true
			return &work, nil
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	// Save the name of the current branch so we can go back.
	save, err := currentBranch()
	if err != nil {
		log.Fatalf("Current branch: %v", err)
	}

	// Find the name of the default branch.
	dbranch, err := defaultBranch(*defBranch, *useRemote)
	if err != nil {
		log.Fatalf("Default branch: %v", err)
	}

	// List local branches that track corresponding remote branches.
	rem, err := listBranchInfo("*", dbranch, *useRemote)
	if err != nil {
		log.Fatalf("Listing branches: %v", err)
	}

	return &workList{
		Base:     dbranch,
		Original: save,
		Remote:   *useRemote,
		Branches: rem,
	}, nil
}
