package main

import (
	"bytes"
	"context"
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
	ID    string
	Name  string
	Token string
}

type Score struct {
	Player *Player
	Score  int
}

type Scoreboard struct {
	Scores []*Score
}

type Answer struct {
	Answer string
}

var SseServer *sse.Server
var rooms []Room
var players []Player

func main() {
	SseServer = sse.New()

	rooms = []Room{
		{
			Name:  "Classic Rock",
			Slug:  "classic-rock",
			Image: "../web/images/guitar.svg",
			Scoreboard: Scoreboard{
				Scores: []*Score{},
			},
		},
		{
			Name:  "Pop Hits",
			Slug:  "pop-hits",
			Image: "../web/images/pop.svg",
			Scoreboard: Scoreboard{
				Scores: []*Score{},
			},
		},
	}

	for _, room := range rooms {
		SseServer.CreateStream(room.Slug)
	}

	r := mux.NewRouter()
	r.Use(authMiddleware)
	r.Use(roomMiddleware)

	r.HandleFunc("/", homeRouteHandler)

	r.HandleFunc("/room/{slug}", gameRouteHandler).Methods("GET")
	r.HandleFunc("/room/{slug}/join", registerPlayerHandler).Methods("POST")
	r.HandleFunc("/room/{slug}/answer", playerAnsweredHandler).Methods("POST")

	r.HandleFunc("/events", SseServer.ServeHTTP)
	r.HandleFunc("/web/{dir}/{filepath:.*}", staticHandler)

	http.Handle("/", r)

	err := http.ListenAndServe("127.0.0.1:3000", r)
	if err != nil {
		log.Fatal(err)
	}
}

func emitScoreboardUpdate(room *Room) {
	var templateBuffer bytes.Buffer
	t, _ := template.ParseFiles("../web/templates/scoreboard.html")
	err := t.Execute(&templateBuffer, room.Scoreboard)
	if err != nil {
		log.Fatal(err)
	}

	SseServer.Publish(room.Slug, &sse.Event{
		Event: []byte("scoreboard-update"),
		Data:  bytes.ReplaceAll(templateBuffer.Bytes(), []byte("\n"), []byte("")),
	})
}

func registerPlayerHandler(w http.ResponseWriter, r *http.Request) {
	room := r.Context().Value("room").(*Room)

	err := r.ParseForm()
	if err != nil {
		fmt.Println(err)
	}

	id := uuid.NewString()
	token := uuid.NewString()

	player := Player{
		ID:    id,
		Name:  r.FormValue("name"),
		Token: token,
	}

	players = append(players, player)

	score := Score{
		Player: &player,
		Score:  0,
	}

	scorePointer := &score

	room.Scoreboard.Scores = append(room.Scoreboard.Scores, scorePointer)

	fmt.Printf("Player %s joined (%s)\n", score.Player.Name, score.Player.Token)

	w.Header().Set("Dbmq-Auth-Token", token)

	emitScoreboardUpdate(room)
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
	player := r.Context().Value("player").(*Player)
	room := r.Context().Value("room").(*Room)

	if player == nil {
		t, _ := template.ParseFiles("../web/templates/base.html", "../web/templates/login.html")
		err := t.Execute(w, room)
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	t, err := template.ParseFiles("../web/templates/base.html", "../web/templates/game.html")

	if err != nil {
		fmt.Println(err)
	}

	err = t.Execute(w, room)
	if err != nil {
		fmt.Println(err)
	}

	room.Scoreboard.findOrCreateScore(player)

	for _, score := range room.Scoreboard.Scores {
		fmt.Printf("Room %s: Player %s has %d points\n", room.Name, player.Name, score.Score)
	}

	emitScoreboardUpdate(room)
}

func playerAnsweredHandler(_ http.ResponseWriter, r *http.Request) {
	player := r.Context().Value("player").(*Player)
	room := r.Context().Value("room").(*Room)

	err := r.ParseForm()
	if err != nil {
		fmt.Println(err)
	}

	answer := Answer{
		Answer: r.FormValue("answer"),
	}

	fmt.Printf("%s answered '%s' in room %s\n", player.Name, answer.Answer, room.Name)

	room.Scoreboard.updateScore(player, 1)

	emitScoreboardUpdate(room)
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

	if token == "" {
		return nil, errors.New("no auth token")
	}

	for i := range players {
		if players[i].Token == token {
			return &players[i], nil
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

func roomMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeParams := mux.Vars(r)
		slug := routeParams["slug"]
		var room = findRoom(slug)

		if slug != "" && room == nil {
			http.Error(w, "Room not found", http.StatusNotFound)
			return
		}

		ctx := context.WithValue(r.Context(), "room", room)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		player, _ := getPlayerFromRequest(r)
		ctx := context.WithValue(r.Context(), "player", player)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Scoreboard) findOrCreateScore(player *Player) *Score {
	for _, score := range s.Scores {
		if score.Player.ID == player.ID {
			return score
		}
	}

	newScore := &Score{
		Player: player,
		Score:  0,
	}

	s.Scores = append(s.Scores, newScore)

	return newScore
}

func (s *Scoreboard) updateScore(player *Player, points int) {
	score := s.findOrCreateScore(player)
	score.Score += points
}
