package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type SnapshotHandler struct {
	Magento *Magento
}

func NewSnapshotHandler(magento *Magento) *SnapshotHandler {
	return &SnapshotHandler{magento}
}

type ListInfo struct {
	Names []Snapshot
}

func (snapshotHandler *SnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	magento := snapshotHandler.Magento
	values := r.URL.Query()
	err := r.ParseForm()
	if err != nil {
		http.Error(w, fmt.Sprint(err), 400)
		return
	}
	if strings.HasPrefix("/take", r.URL.Path) {
		message := r.PostForm.Get("message")
		magento.TakeSnapshot(message)
		http.Redirect(w, r, "/list", 302)
		return
	} else if strings.HasPrefix("/list", r.URL.Path) {
		names, err := magento.ListSnapshots()
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
		names, err := magento.ListSnapshots()
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
			return
		}

		diffRevs, err := GetDiffRevs(values.Get("ss1"), values.Get("ss2"), len(names))
		if err != nil {
			http.Error(w, fmt.Sprint(err), 400)
			return
		}

		diff, err := magento.Diff(names[diffRevs.Old].Name, names[diffRevs.New].Name, diffRevs.Old, diffRevs.New)

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
		names, err := magento.ListSnapshots()
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

		diff, err := magento.Diff(names[diffRevs.Old].Name, names[diffRevs.New].Name, diffRevs.Old, diffRevs.New)
		w.Header().Add("Content-Type", "text/plain")
		for _, r := range diff.Lines {
			value := strings.Replace(r.NewValue, "\n", "\\n", -1)
			fmt.Fprintf(w, `config:set --scope="%s" --scope-id="%d" "%s" "%s"`,
				r.Scope, r.ScopeId, r.Path, value)
			fmt.Fprintln(w, "")
		}
		return
	}
	http.NotFound(w, r)
}
