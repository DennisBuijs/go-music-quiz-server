package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/r3labs/sse/v2"
)

type Answer struct {
	Answer string
}

type Player struct {
	Name string
}

type Score struct {
	Player Player
	Score  int
}

type Scoreboard struct {
	Scores []Score
}

var SseServer *sse.Server
var scoreboard Scoreboard

func main() {
	SseServer = sse.New()
	SseServer.CreateStream("scoreboard")

	var players []Player
	scoreboard = Scoreboard{}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t, err := template.ParseFiles("../web/templates/base.html", "../web/templates/home.html")

		if err != nil {
			fmt.Println(err)
		}

		err = t.Execute(w, nil)
		if err != nil {
			fmt.Println(err)
		}
	})

	http.HandleFunc("/quiz", func(w http.ResponseWriter, r *http.Request) {
		t, err := template.ParseFiles("../web/templates/base.html", "../web/templates/game.html")

		if err != nil {
			fmt.Println(err)
		}

		err = t.Execute(w, nil)
		if err != nil {
			fmt.Println(err)
		}
	})

	http.HandleFunc("/player/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseForm()
		if err != nil {
			fmt.Println(err)
		}

		var player Player
		player.Name = r.FormValue("name")

		players = append(players, player)

		var score Score
		score.Player = player
		score.Score = 0

		scoreboard.Scores = append(scoreboard.Scores, score)

		emitScoreboardUpdate()
	})

	http.HandleFunc("/answer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// r.ParseForm()

		// var answer Answer
		// answer.Answer = r.FormValue("answer")
	})

	http.HandleFunc("/web/", staticHandler)

	http.HandleFunc("/events", SseServer.ServeHTTP)

	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		log.Fatal(err)
	}
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
		log.Fatal(err)
	}
}

func emitScoreboardUpdate() {
	var templateBuffer bytes.Buffer
	t, _ := template.ParseFiles("../web/templates/scoreboard.html")
	err := t.Execute(&templateBuffer, scoreboard)
	if err != nil {
		log.Fatal(err)
	}

	SseServer.Publish("scoreboard", &sse.Event{
		Data: bytes.ReplaceAll(templateBuffer.Bytes(), []byte("\n"), []byte("")),
	})
}
