package main

import (
	"encoding/json"
	"fmt"
	"github.com/pstuifzand/magereport/backend"
	"net/http"
	"strings"
)

type SnapshotHandler struct {
	Source  backend.SourceBackend
	Backend backend.Backend
}

func NewSnapshotHandler(s backend.SourceBackend, b backend.Backend) *SnapshotHandler {
	return &SnapshotHandler{s, b}
}

type ListInfo struct {
	Names []backend.Snapshot
}

func (snapshotHandler *SnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	magento := snapshotHandler.Source
	fb := snapshotHandler.Backend
	values := r.URL.Query()
	err := r.ParseForm()
	if err != nil {
		http.Error(w, fmt.Sprint(err), 400)
		return
	}
	if strings.HasPrefix("/take", r.URL.Path) {
		message := r.PostForm.Get("message")
		vars, err := magento.TakeSnapshot(message)
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
			return
		}
		fb.SaveSnapshot(vars)
		http.Redirect(w, r, "/list", 302)
		return
	} else if strings.HasPrefix("/list", r.URL.Path) {
		names, err := fb.ListSnapshots()
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
		}
		accept := r.Header.Get("Accept")
		if values.Get("format") == "json" || strings.Contains(accept, "application/json") {
			w.Header().Add("Content-Type", "application/json")
			json.NewEncoder(w).Encode(names)
		} else {

			if err != nil {
				http.Error(w, fmt.Sprint(err), 500)
			} else {
				err = TT.ExecuteTemplate(w, "List", ListInfo{names})
				if err != nil {
					http.Error(w, fmt.Sprint(err), 500)
				}
			}
		}
		return
	} else if strings.HasPrefix("/diff", r.URL.Path) {
		names, err := fb.ListSnapshots()
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
			return
		}

		diffRevs, err := GetDiffRevs(values.Get("ss1"), values.Get("ss2"), len(names))
		if err != nil {
			http.Error(w, fmt.Sprint(err), 400)
			return
		}

		oldSnapshot, err := fb.LoadSnapshot(names[diffRevs.Old].Name)
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
			return
		}
		newSnapshot, err := fb.LoadSnapshot(names[diffRevs.New].Name)
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
			return
		}

		diff, err := backend.Diff(oldSnapshot, newSnapshot, diffRevs.Old, diffRevs.New)

		accept := r.Header.Get("Accept")
		if values.Get("format") == "json" || strings.Contains(accept, "application/json") {
			w.Header().Add("Content-Type", "application/json")
			json.NewEncoder(w).Encode(diff)
			return
		} else {
			if err != nil {
				http.Error(w, fmt.Sprint(err), 500)
				return
			} else {
				err = TT.ExecuteTemplate(w, "Diff", diff)
				if err != nil {
					http.Error(w, fmt.Sprint(err), 500)
					return
				}
			}
		}
		return
	} else if strings.HasPrefix("/export", r.URL.Path) {
		names, err := fb.ListSnapshots()
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
			return
		}

		values := r.URL.Query()
		diffRevs, err := GetDiffRevs(values.Get("ss1"), values.Get("ss2"), len(names))
		if err != nil {
			http.Error(w, fmt.Sprint(err), 400)
			return
		}

		oldSnapshot, err := fb.LoadSnapshot(names[diffRevs.Old].Name)
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
			return
		}

		newSnapshot, err := fb.LoadSnapshot(names[diffRevs.New].Name)
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
			return
		}

		w.Header().Add("Content-Type", "text/plain")

		err = backend.Export(w, oldSnapshot, newSnapshot, diffRevs.Old, diffRevs.New)
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
			return
		}

		return
	}
	http.NotFound(w, r)
}
