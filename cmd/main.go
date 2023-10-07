package main

import (
	"bytes"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/r3labs/sse/v2"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

type Room struct {
	Name       string
	Slug       string
	Image      string
	Scoreboard Scoreboard
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
var rooms []Room

func main() {
	SseServer = sse.New()
	SseServer.CreateStream("scoreboard")

	rooms = []Room{
		{
			Name:  "Classic Rock",
			Slug:  "classic-rock",
			Image: "../web/images/guitar.svg",
			Scoreboard: Scoreboard{
				Scores: []Score{},
			},
		},
		{
			Name:  "Pop Hits",
			Slug:  "pop-hits",
			Image: "../web/images/pop.svg",
			Scoreboard: Scoreboard{
				Scores: []Score{},
			},
		},
	}

	r := mux.NewRouter()

	r.HandleFunc("/", homeRouteHandler)

	r.HandleFunc("/room/{slug}", gameRouteHandler)
	r.HandleFunc("/room/{slug}/join", registerPlayerHandler)

	r.HandleFunc("/events", SseServer.ServeHTTP)
	r.HandleFunc("/web/{dir}/{filepath:.*}", staticHandler)

	http.Handle("/", r)

	err := http.ListenAndServe("127.0.0.1:3000", r)
	if err != nil {
		log.Fatal(err)
	}
}

func emitScoreboardUpdate(scoreboard Scoreboard) {
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

func registerPlayerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	routeParams := mux.Vars(r)
	slug := routeParams["slug"]
	var room = findRoom(slug)

	if room == nil {
		http.Error(w, "Room not found", http.StatusNotFound)
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Println(err)
	}

	var player Player
	player.Name = r.FormValue("name")

	var score Score
	score.Player = player
	score.Score = 0

	room.Scoreboard.Scores = append(room.Scoreboard.Scores, score)

	emitScoreboardUpdate(room.Scoreboard)
}

func findRoom(slug string) *Room {
	for i := range rooms {
		if rooms[i].Slug == slug {
			return &rooms[i]
		}
	}

	return nil
}

func homeRouteHandler(w http.ResponseWriter, _ *http.Request) {
	t, err := template.ParseFiles("../web/templates/base.html", "../web/templates/home.html")

	if err != nil {
		fmt.Println(err)
	}

	err = t.Execute(w, rooms)
	if err != nil {
		fmt.Println(err)
	}
}

func gameRouteHandler(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("../web/templates/base.html", "../web/templates/game.html")

	if err != nil {
		fmt.Println(err)
	}

	routeParams := mux.Vars(r)
	slug := routeParams["slug"]
	var room = findRoom(slug)

	err = t.Execute(w, room)
	if err != nil {
		fmt.Println(err)
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
