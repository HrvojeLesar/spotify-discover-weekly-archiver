package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/zmb3/spotify/v2"
	"github.com/zmb3/spotify/v2/auth"
)

const host = "localhost"

var (
    state = "amazinglysecurestate"
	ch = make(chan *spotify.Client)

)

func makeSpotifyAuth() *spotifyauth.Authenticator {
	port := os.Getenv("DWSPort")
    redirectURI := fmt.Sprintf("http://%s:%s/callback", host, port)
	return spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(
            spotifyauth.ScopeUserLibraryRead,
            spotifyauth.ScopePlaylistReadPrivate,
            spotifyauth.ScopeUserReadPrivate))
}

func makeHttpServer() {
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})

	go func() {
		port := os.Getenv("DWSPort")
		address := fmt.Sprintf("%s:%s", host, port)
		err := http.ListenAndServe(address, nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	auth := makeSpotifyAuth()
	url := auth.AuthURL(state)
	fmt.Println("Log in to spotify by visiting: ", url)

	client := <-ch

    user, err := client.CurrentUser(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("You are logged in as:", user.DisplayName)

    playlists, err := client.CurrentUsersPlaylists(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    for _, playlist := range playlists.Playlists {
        fmt.Println("-----------------------")
        fmt.Println("Name:", playlist.Name)
        fmt.Println("Owner:", playlist.Owner)
    }
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
    auth := makeSpotifyAuth()
    token, err := auth.Token(r.Context(), state, r)
    if err != nil {
        http.Error(w, "Couldn't get token", http.StatusForbidden)
        log.Fatal(err)
    }
    if st := r.FormValue("state"); st != state {
        http.NotFound(w, r)
        log.Fatalf("State mismatch: %s != %s\n", st, state)
    }

    client := spotify.New(auth.Client(r.Context(), token))
    fmt.Fprintf(w, "Login completed")
    ch <- client
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
    makeHttpServer()
}
