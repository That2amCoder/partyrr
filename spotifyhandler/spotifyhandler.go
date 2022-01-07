package SpotifyHandle

import (
	"context"
	"fmt"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

type SpotifyHandle struct {
	token *oauth2.Token
	sp    *spotify.Client
}

//make a initialiser function
func NewSpotifyHandle(token *oauth2.Token) *SpotifyHandle {
	ctx := context.Background()
	httpClient := spotifyauth.New().Client(ctx, token)
	client := spotify.New(httpClient)
	return &SpotifyHandle{
		token: token,
		sp:    client,
	}
}

func (s *SpotifyHandle) GetPlaylist(playlistID string) []spotify.PlaylistTrack {
	ctx := context.Background()

	// Get the playlist from the spotify api
	//cast the playlistID to a spotify.ID
	playlistID2 := spotify.ID(playlistID)

	playlist, err := s.sp.GetPlaylistTracks(ctx, playlistID2)
	if err != nil {
		fmt.Println(err)
	}
	// For the first 10 songs return in a list the song name and the artist

	playlistres := playlist.Tracks

	return playlistres
}

func (s *SpotifyHandle) CreatePlaylistID(playlistName string) string {
	ctx := context.Background()

	httpClient := spotifyauth.New().Client(ctx, s.token)
	client := spotify.New(httpClient)
	user, err := client.CurrentUser(ctx)
	if err != nil {
		fmt.Println(err)
	}
	//print userid
	fmt.Println(user.ID)
	playlistID, err := client.CreatePlaylistForUser(ctx, user.ID, playlistName, "", false, false)
	if err != nil {
		fmt.Println(err)
	}
	//cast the playlistID to a string
	playlistID2 := string(playlistID.ID)
	return playlistID2
}

func (s *SpotifyHandle) AddSong(playlistID string, songName string) {
	ctx := context.Background()

	// Add a song to the playlist
	// Search for the song in the spotify api
	results, err := s.sp.Search(ctx, songName, spotify.SearchTypeTrack)
	if err != nil {
		fmt.Println(err)
	}
	// Get the first result from results
	track := results.Tracks.Tracks[0]
	// Add the song to the playlist using its URI
	//func (c *Client) AddTracksToPlaylist(playlistID ID, trackIDs ...ID) (snapshotID string, err error)

	s.sp.AddTracksToPlaylist(ctx, spotify.ID(playlistID), track.ID)
}
