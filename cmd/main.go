package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/r3labs/sse/v2"
)

type Answer struct {
	Answer string
}

func main() {
	sseServer := sse.New()
	sseServer.CreateStream("scoreboard")

	var PreviousAnswers []string

	http.HandleFunc("/quiz", func(w http.ResponseWriter, r *http.Request) {
		t, err := template.ParseFiles("../web/templates/index.html")

		if err != nil {
			fmt.Println(err)
		}

		t.Execute(w, nil)
	})

	http.HandleFunc("/answer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allows", http.StatusMethodNotAllowed)
		}

		r.ParseForm()

		var answer Answer
		answer.Answer = r.FormValue("answer")

		PreviousAnswers = append(PreviousAnswers, answer.Answer)

		var templateBuffer bytes.Buffer
		t, _ := template.ParseFiles("../web/templates/scoreboard.html")
		t.Execute(&templateBuffer, struct{ Answers []string }{Answers: PreviousAnswers})

		sseServer.Publish("scoreboard", &sse.Event{
			Data: bytes.ReplaceAll(templateBuffer.Bytes(), []byte("\n"), []byte("")),
		})
	})

	http.HandleFunc("/web/", staticHandler)

	http.HandleFunc("/events", sseServer.ServeHTTP)

	http.ListenAndServe(":3000", nil)
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasSuffix(path, "js") {
		w.Header().Set("Content-Type", "text/javascript")
	} else if strings.HasSuffix(path, "css") {
		w.Header().Set("Content-Type", "text/css")
	} else {
		w.Header().Set("Content-type", "image/svg+xml")
	}
	data, err := os.ReadFile("../" + path[1:])
	if err != nil {
		fmt.Print(err)
	}
	_, err = w.Write(data)
	if err != nil {
		fmt.Print(err)
	}
}
