//Create a new file called party.go

package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	dbhandler "partyrr/database"
	SpotifyHandle "partyrr/spotifyhandler"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"github.com/skip2/go-qrcode"
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
		invitecode := r.FormValue("partyID")
		//try to convert the invitecode to the partyID
		partyID, err := dbhandle.GetPartyID(invitecode)
		if err != nil {
			fmt.Printf("Error converting partyID to int\n")
			http.Error(w, "Invalid partyID", http.StatusBadRequest)
		}

		tok, err := dbhandle.Getoath(partyID)
		fmt.Println(tok.RefreshToken)
		if err != nil {
			fmt.Printf("Error getting oauth token\n")
			http.Error(w, "Invalid partyID", http.StatusBadRequest)
		}

		spotifyhndl := SpotifyHandle.NewSpotifyHandle(tok)
		playlistID, _ := dbhandle.GetPlaylist(partyID)

		spotifyhndl.AddSong(playlistID, songName)
		http.Redirect(w, r, "/party/"+invitecode, http.StatusTemporaryRedirect)
	})

	r.HandleFunc("/party/{invlink}", func(w http.ResponseWriter, r *http.Request) {
		// Create the party
		invlink := mux.Vars(r)["invlink"]
		_, err := dbhandle.GetPartyID(invlink)
		if err != nil {
			fmt.Printf("%s\n", err)
			http.Error(w, "Invalid link", http.StatusBadRequest)
			return
		}

		//serve the file
		http.ServeFile(w, r, "./templates/party.html")
	}).Methods("GET", "POST")

	//Make a api call to get the user's playlists
	r.HandleFunc("/party/{invlink}/songs", func(w http.ResponseWriter, r *http.Request) {
		// Get the name and host from the request
		invlink := mux.Vars(r)["invlink"]

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

		//return the playlist in a json format to the caller
		json.NewEncoder(w).Encode(playlist)

	}).Methods("GET", "POST")

	r.HandleFunc("/party/{invlink}/qr", func(w http.ResponseWriter, r *http.Request) {
		// Get the name and host from the request
		invlink := mux.Vars(r)["invlink"]

		qrCode, err := qrcode.Encode("http://"+address+"/party/"+invlink, qrcode.Medium, 256)

		if err != nil {
			log.Fatal(err)
		}

		//return the qrcode image
		w.Header().Set("Content-Type", "image/png")
		w.Write(qrCode)

	}).Methods("GET", "POST")

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
