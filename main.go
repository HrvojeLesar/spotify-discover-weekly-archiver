package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/joho/godotenv"
	"github.com/zmb3/spotify/v2"
	"github.com/zmb3/spotify/v2/auth"
)

// try to find playlist by name, same songs
// make playlist from discover weekly if not found

const DISCOVER_WEEKLY_PLAYLIST_NAME = "Discover Weekly"
const DISCOVER_WEEKLY_PLAYLIST_OWNER_ID = "spotify"
const DISCOVER_WEEKLY_PLAYLIST_TRACKS = 30

type Cache struct {
	currentDiscoverWeeklyPlaylist *spotify.SimplePlaylist
	currentDiscoverWeeklyTracks   *spotify.PlaylistTrackPage
	currentUser                   *spotify.PrivateUser
}

type SpotifyClient struct {
	host          string
	state         string
	port          string
	channel       chan *spotify.Client
	client        *spotify.Client
	authenticator *spotifyauth.Authenticator
	ctx           context.Context
	cache         Cache
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
		cache: Cache{
			currentDiscoverWeeklyTracks:   nil,
			currentDiscoverWeeklyPlaylist: nil,
			currentUser:                   nil,
		},
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
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopePlaylistModifyPublic,
			spotifyauth.ScopePlaylistModifyPrivate))
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

	sc.cache.currentUser = user

	// finds if already archived
	sc.findDiscoverWeekly()
	if sc.isDWNotArchived() {
		log.Println("Not archived. Archiving...")
        sc.archivePlaylist()
	} else {
		log.Println("Already archived.")
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

func (sc *SpotifyClient) findDiscoverWeekly() {
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

	sc.cache.currentDiscoverWeeklyPlaylist = discoverWeekly

	tracks, err := sc.client.GetPlaylistTracks(sc.ctx, sc.cache.currentDiscoverWeeklyPlaylist.ID, spotify.Limit(30))
	if err != nil {
		log.Fatal(err)
	}

	sc.cache.currentDiscoverWeeklyTracks = tracks
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

func (sc *SpotifyClient) isDWNotArchived() bool {
	if sc.cache.currentDiscoverWeeklyPlaylist == nil {
		log.Fatal("There is no current discover weekly playlist")
	}
	playlists, err := sc.client.CurrentUsersPlaylists(sc.ctx)
	if err != nil {
		log.Fatal(err)
	}

	sortedDWTracks := make([]spotify.PlaylistTrack, 30)
	copy(sortedDWTracks, sc.cache.currentDiscoverWeeklyTracks.Tracks)
	sort.Slice(sortedDWTracks[:], func(i, j int) bool {
		return sortedDWTracks[i].Track.ID > sortedDWTracks[j].Track.ID
	})

	for page := 1; ; page++ {
		for _, playlist := range playlists.Playlists {
			if (playlist.Name == DISCOVER_WEEKLY_PLAYLIST_NAME &&
				playlist.Owner.ID == DISCOVER_WEEKLY_PLAYLIST_OWNER_ID) ||
				playlist.Tracks.Total != DISCOVER_WEEKLY_PLAYLIST_TRACKS {
				continue
			}

			tracks, err := sc.client.GetPlaylistTracks(sc.ctx, sc.cache.currentDiscoverWeeklyPlaylist.ID, spotify.Limit(30))
			if err != nil {
				log.Fatal(err)
			}

			sort.Slice(tracks.Tracks[:], func(i, j int) bool {
				return tracks.Tracks[i].Track.ID > tracks.Tracks[j].Track.ID
			})

			for i := 0; i < 30; i++ {
				if sortedDWTracks[i].Track.ID != tracks.Tracks[i].Track.ID {
					break
				}
				return false
			}
		}
		err = sc.client.NextPage(sc.ctx, playlists)
		if err == spotify.ErrNoMorePages {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
	}

	return true
}

func (sc *SpotifyClient) archivePlaylist() {
    dt := time.Now()
    archivePlaylistName := dt.Format("02-01-06") + " DW"
    playlist, err := sc.client.CreatePlaylistForUser(sc.ctx, sc.cache.currentUser.ID, archivePlaylistName, "", false, false)
	if err != nil {
		log.Fatal(err)
	}

    trackIds := make([]spotify.ID, 30)
    for i, track := range sc.cache.currentDiscoverWeeklyTracks.Tracks {
        trackIds[i] = track.Track.ID
    }

    _, err = sc.client.AddTracksToPlaylist(sc.ctx, playlist.ID, trackIds...)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// spotifyClient := newSpotifyClient()
	newSpotifyClient()
}
