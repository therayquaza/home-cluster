package main

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"sync"
)

type Note struct {
	ID      int
	Message string
}

var (
	notes   = []Note{}
	counter = 0
	mu      sync.Mutex
)

func handler(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.ParseFiles(filepath.Join("assets", "index.html"))

	mu.Lock()
	defer mu.Unlock()

	if r.Method == http.MethodPost {
		msg := r.FormValue("message")
		if msg != "" {
			notes = append([]Note{{ID: counter, Message: msg}}, notes...)
			counter++
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	_ = tmpl.Execute(w, map[string]interface{}{"Notes": notes})
}

// New Delete Endpoint
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	id := r.FormValue("id")

	mu.Lock()
	for i, n := range notes {
		if fmt.Sprintf("%d", n.ID) == id {
			notes = append(notes[:i], notes[i+1:]...)
			break
		}
	}
	mu.Unlock()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/delete", deleteHandler)
	fmt.Println("⚡ PixelVault OS: http://localhost:8080")
	_ = http.ListenAndServe(":8080", nil)
}

