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

// find discover weekly playlist
// try to find playlist by name, same songs
// make playlist from discover weekly if not found

const DISCOVER_WEEKLY_PLAYLIST_NAME = "Discover Weekly"
const DISCOVER_WEEKLY_PLAYLIST_OWNER_ID = "spotify"

type SpotifyClient struct {
	host          string
	state         string
	port          string
	channel       chan *spotify.Client
	client        *spotify.Client
	authenticator *spotifyauth.Authenticator
	ctx           context.Context
}

func newSpotifyClient() *SpotifyClient {
	c := &SpotifyClient{
		host:          "localhost",
		state:         "amazinglysecurestate",
		port:          os.Getenv("DWSPort"),
		channel:       make(chan *spotify.Client),
		client:        nil,
		authenticator: nil,
		ctx:           context.Background(),
	}

	c.newAuthenticatorWithScopes()
	c.startAuth()

	return c
}

func (sc *SpotifyClient) newAuthenticatorWithScopes() {
	redirectURI := fmt.Sprintf("http://%s:%s/callback", sc.host, sc.port)
	sc.authenticator = spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserLibraryRead,
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopeUserReadPrivate))
}

func (sc *SpotifyClient) startAuth() {
	http.HandleFunc("/callback", sc.completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})

	go func() {
		address := fmt.Sprintf("%s:%s", sc.host, sc.port)
		err := http.ListenAndServe(address, nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	fmt.Println("Log in to spotify by visiting: ", sc.authenticator.AuthURL(sc.state))

	<-sc.channel

	user, err := sc.client.CurrentUser(sc.ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("You are logged in as:", user.DisplayName)

	playlists, err := sc.client.CurrentUsersPlaylists(sc.ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, playlist := range playlists.Playlists {
		fmt.Println("-----------------------")
		fmt.Println("Name:", playlist.Name)
		fmt.Println("Owner:", playlist.Owner.ID)
	}
}

func (sc *SpotifyClient) completeAuth(w http.ResponseWriter, r *http.Request) {
	token, err := sc.authenticator.Token(r.Context(), sc.state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != sc.state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, sc.state)
	}

	sc.client = spotify.New(sc.authenticator.Client(r.Context(), token))
	fmt.Fprintf(w, "Login completed")
	sc.channel <- sc.client
}

func (sc *SpotifyClient) findDiscoverWeekly() *spotify.SimplePlaylist {
	playlists, err := sc.client.CurrentUsersPlaylists(sc.ctx)
	if err != nil {
		log.Fatal(err)
	}

	discoverWeekly := getDiscoverWeeklyPlaylistFromPlaylists(playlists)
    if discoverWeekly == nil {
        for page := 1; ; page++ {
            discoverWeekly = getDiscoverWeeklyPlaylistFromPlaylists(playlists)
            if discoverWeekly != nil {
                break
            }
            err = sc.client.NextPage(sc.ctx, playlists)
            if err == spotify.ErrNoMorePages {
                break
            }
            if err != nil {
                log.Fatal(err)
            }
        }
    }

    if discoverWeekly == nil {
        log.Fatal("Failed to find discover weekly playlist.")
    }

    return discoverWeekly
}

func getDiscoverWeeklyPlaylistFromPlaylists(playlists *spotify.SimplePlaylistPage) *spotify.SimplePlaylist {
	for _, playlist := range playlists.Playlists {
		if playlist.Name == DISCOVER_WEEKLY_PLAYLIST_NAME &&
			playlist.Owner.ID == DISCOVER_WEEKLY_PLAYLIST_OWNER_ID {
			return &playlist
		}
	}
	return nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// spotifyClient := newSpotifyClient()
	newSpotifyClient()
}
