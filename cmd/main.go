package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/google/uuid"
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
	Name  string
	Token string
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
	r.Use(loggingMiddleware)

	r.HandleFunc("/", homeRouteHandler)

	r.HandleFunc("/room/{slug}", gameRouteHandler).Methods("GET")
	r.HandleFunc("/room/{slug}/join", registerPlayerHandler).Methods("POST")

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

	token := uuid.NewString()

	player := Player{
		Name:  r.FormValue("name"),
		Token: token,
	}

	var score Score
	score.Player = player
	score.Score = 0

	room.Scoreboard.Scores = append(room.Scoreboard.Scores, score)

	fmt.Printf("Player %v joined (%s)", score.Player.Name, score.Player.Token)

	w.Header().Set("Dbmq-Auth-Token", token)

	emitScoreboardUpdate(room.Scoreboard)
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
	player, _ := getPlayerFromRequest(r)

	routeParams := mux.Vars(r)
	slug := routeParams["slug"]
	var room = findRoom(slug)

	if player == nil {
		t, _ := template.ParseFiles("../web/templates/base.html", "../web/templates/login.html")
		err := t.Execute(w, room)
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	fmt.Println(player.Name)

	t, err := template.ParseFiles("../web/templates/base.html", "../web/templates/game.html")

	if err != nil {
		fmt.Println(err)
	}

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

func getPlayerFromRequest(r *http.Request) (*Player, error) {
	token := r.Header.Get("Authentication-Token")

	fmt.Println(token)

	routeParams := mux.Vars(r)
	slug := routeParams["slug"]
	var room = findRoom(slug)

	for i := range room.Scoreboard.Scores {
		if room.Scoreboard.Scores[i].Player.Token == token {
			fmt.Println(room.Scoreboard.Scores[i].Player)
			return &room.Scoreboard.Scores[i].Player, nil
		}
	}

	return nil, errors.New("player not found")
}

func findRoom(slug string) *Room {
	for i := range rooms {
		if rooms[i].Slug == slug {
			return &rooms[i]
		}
	}

	return nil
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Header.Get("Authentication-Token"))
		next.ServeHTTP(w, r)
	})
}
