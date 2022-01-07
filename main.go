//Create a new file called party.go

package main

import (
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	dbhandler "partyrr/src/database"
	SpotifyHandle "partyrr/src/spotifyhandler"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"github.com/skip2/go-qrcode"
	spotify "github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

var (
	address = os.Getenv("ADDRESS")
	state   = "partyrr_auth"
	key     = []byte("super-secret-key")
	store   = sessions.NewCookieStore(key)
)

func main() {

	dbhandle := dbhandler.NewPartyDB()
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
	}

	//TODO: Clear up the code below
	address = os.Getenv("ADDRESS")
	redirectURI := "http://" + address + "/callback"
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	auth := spotifyauth.New(spotifyauth.WithClientID(clientID), spotifyauth.WithClientSecret(clientSecret), spotifyauth.WithRedirectURL(redirectURI), spotifyauth.WithScopes(spotifyauth.ScopeUserReadPrivate, spotifyauth.ScopePlaylistModifyPrivate))

	// Create a new router
	r := mux.NewRouter()

	// Attach an elegant path with handler
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./templates/index.html")
	})

	// Spotify token endpoint
	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		url := auth.AuthURL(state)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})

	//Handle the callback from Spotify after login
	r.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Received callback from Spotify!\n")

		tok, err := auth.Token(r.Context(), state, r)
		gob.Register(tok)

		if err != nil {
			http.Error(w, "Couldn't get token", http.StatusForbidden)
			log.Fatal(err)
		}
		if st := r.FormValue("state"); st != state {
			http.NotFound(w, r)
			log.Fatalf("State mismatch: %s != %s\n", st, state)
		}

		// store the token in the session
		session, err := store.Get(r, "partyrr_session")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		session.Values["token"] = tok
		err = session.Save(r, w)

		if err != nil {
			fmt.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		//redirect the user to index
		http.Redirect(w, r, "/partycreator", http.StatusTemporaryRedirect)
	})

	// Create party endpoint
	r.HandleFunc("/createParty", func(w http.ResponseWriter, r *http.Request) {
		// get the name and host from the request
		name := r.FormValue("name")
		host := r.FormValue("host")

		//get client token
		session, err := store.Get(r, "partyrr_session")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tok := session.Values["token"].(*oauth2.Token)

		// Create the party
		fmt.Printf("Creating party with name: %s and host: %s\n", name, host)

		//print current username
		fmt.Printf("Current user: %s\n", tok.AccessToken)

		//save the token
		tokenID, err := dbhandle.SaveToken(tok)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		partyID, err := dbhandle.CreateParty(name, host, tokenID)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		spotifyhndl := SpotifyHandle.NewSpotifyHandle(tok)

		//print getting client
		playlistID := spotifyhndl.CreatePlaylistID(name + " Party")

		//Link the playlist to the party
		err = dbhandle.CreateQueue(partyID, playlistID)

		if err != nil {
			fmt.Printf("%s\n", err)
		}

		//Generate an invite link
		invlink, _ := dbhandle.GetInvitelink(partyID)

		http.Redirect(w, r, "/party/"+invlink, http.StatusTemporaryRedirect)
	})

	r.HandleFunc("/party/addsong", func(w http.ResponseWriter, r *http.Request) {
		// Get the name and host from the request
		songName := r.FormValue("song")
		partyID := r.FormValue("partyID")
		//try to convert the partID to int
		partyIDInt, err := strconv.Atoi(partyID)
		if err != nil {
			fmt.Printf("Error converting partyID to int\n")
			http.Error(w, "Invalid partyID", http.StatusBadRequest)
		}

		tok, err := dbhandle.Getoath(partyIDInt)

		if err != nil {
			fmt.Printf("Error getting oauth token\n")
			http.Error(w, "Invalid partyID", http.StatusBadRequest)
		}

		spotifyhndl := SpotifyHandle.NewSpotifyHandle(tok)

		playlistID, _ := dbhandle.GetPlaylist(partyIDInt)
		spotifyhndl.AddSong(playlistID, songName)
		partylink, _ := dbhandle.GetInvitelink(partyIDInt)
		http.Redirect(w, r, "/party/"+partylink, http.StatusTemporaryRedirect)
	})

	r.HandleFunc("/party/{invlink}", func(w http.ResponseWriter, r *http.Request) {
		// Create the party
		invlink := mux.Vars(r)["invlink"]
		fmt.Printf("Invite link: %s\n", invlink)
		partyID, err := dbhandle.GetPartyID(invlink)
		if err != nil {
			fmt.Printf("%s\n", err)
			http.Error(w, "Invalid link", http.StatusBadRequest)
			return
		}

		tok, err := dbhandle.Getoath(partyID)
		if err != nil {
			//print the error
			fmt.Printf("%s\n", err)
			http.Error(w, "Invalid link", http.StatusBadRequest)
			return
		}

		playlistID, err := dbhandle.GetPlaylist(partyID)
		if err != nil {
			//print the error
			fmt.Printf("%s\n", err)
			http.Error(w, "Broken link", http.StatusBadRequest)
			return
		}

		spotifyhndl := SpotifyHandle.NewSpotifyHandle(tok)
		playlist := spotifyhndl.GetPlaylist(playlistID)

		// Generate a new qr code to a buffer and convert it to base64
		qrCode, err := qrcode.Encode("http://"+address+"/party/"+invlink, qrcode.Medium, 256)

		if err != nil {
			log.Fatal(err)
		}

		qrCodeBase64 := base64.StdEncoding.EncodeToString(qrCode)
		tpl := template.Must(template.ParseFiles("./templates/party.html"))
		//initialise a return struct to pass to the template
		data := struct {
			PartyID  int
			Playlist []spotify.PlaylistTrack
			QRCode   string
		}{
			PartyID:  partyID,
			Playlist: playlist,
			QRCode:   qrCodeBase64,
		}
		// Serve the party.html template found in /templates
		tpl.ExecuteTemplate(w, "party.html", data)
		//
	}).Methods("GET", "POST")

	r.HandleFunc("/joinParty", func(w http.ResponseWriter, r *http.Request) {
		// get the partyID from the request
		inviteCode := r.FormValue("inviteCode")
		//TODO add statistics
		//redirect to the party
		http.Redirect(w, r, "/party/"+inviteCode, http.StatusTemporaryRedirect)
	}).Methods("POST")

	r.HandleFunc("/partycreator", func(w http.ResponseWriter, r *http.Request) {
		// Render the "partycreator.html" template
		tpl := template.Must(template.ParseFiles("./templates/partycreator.html"))
		tpl.ExecuteTemplate(w, "partycreator.html", nil)
	}).Methods("GET")

	// Create a CORS object to allow cross-origin requests
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://" + address},
		AllowCredentials: true,
	})

	// Create a negroni handler
	n := c.Handler(r)

	// Start the server
	fmt.Printf("Starting server on %s\n", address)
	http.ListenAndServe(address, n)
}