package dbhandler

import (
	"database/sql"
	"log"
	"math/rand"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
)

type PartyDB struct {
	db *sql.DB
}

func NewPartyDB() *PartyDB {
	db, err := sql.Open("sqlite3", "partyrr.db")
	if err != nil {

		log.Fatal(err)
	}
	db.Exec("CREATE TABLE IF NOT EXISTS parties (id INTEGER PRIMARY KEY, name TEXT, host TEXT, invitecode TEXT, oath_token_id INTEGER)")
	db.Exec("CREATE TABLE IF NOT EXISTS que (partyID INTEGER, playlist TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS oath (id INTEGER PRIMARY KEY, access_token TEXT, refresh_token TEXT, expiry TEXT)")

	return &PartyDB{db: db}
}

func (p *PartyDB) CreateParty(name string, host string, oath int64) (int, error) {
	conn := p.db
	invitelink := generateInvitecode(4)
	//prepare the statement
	stmt, err := conn.Prepare("INSERT INTO parties (name, host, invitecode, oath_token_id) VALUES (?,?,?,?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	//execute the statement
	res, err := stmt.Exec(name, host, invitelink, oath)
	if err != nil {
		return 0, err
	}
	//get the id of the party
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (p *PartyDB) CreateQueue(partyID int, playlistlnk string) error {
	conn := p.db
	//insert statement
	stmt, err := conn.Prepare("INSERT INTO que (partyID, playlist) VALUES (?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	//execute the statement
	_, err = stmt.Exec(partyID, playlistlnk)
	if err != nil {
		return err
	}
	return nil
}

func (p *PartyDB) GetPlaylist(partyID int) (string, error) {
	row, err := p.db.Query("SELECT playlist FROM que WHERE partyID = ?", partyID)
	if err != nil {
		return "", err
	}
	defer row.Close()
	var playlist string
	row.Next()
	err = row.Scan(&playlist)
	if err != nil {
		return "", err
	}
	return playlist, nil
}

func (p *PartyDB) GetPartyID(invitecode string) (int, error) {
	row, err := p.db.Query("SELECT id FROM parties WHERE invitecode = ?", invitecode)
	if err != nil {
		return 0, err
	}
	defer row.Close()
	var partyID int
	row.Next()
	err = row.Scan(&partyID)
	if err != nil {
		return 0, err
	}
	return partyID, nil
}

func (p *PartyDB) GetInvitelink(partyID int) (string, error) {
	row, err := p.db.Query("SELECT invitecode FROM parties WHERE id = ?", partyID)
	if err != nil {
		return "", err
	}
	defer row.Close()
	var invitelink string
	row.Next()
	err = row.Scan(&invitelink)
	if err != nil {
		return "", err
	}
	return invitelink, nil
}

func (p *PartyDB) Getoath(partyID int) (*oauth2.Token, error) {
	row, err := p.db.Query("SELECT access_token, refresh_token, expiry FROM oath WHERE id = ?", partyID)
	if err != nil {
		return nil, err
	}
	//defer row.Close()
	var access_token string
	var refresh_token string
	var expiry string
	var expiryT time.Time
	row.Next()
	err = row.Scan(&access_token, &refresh_token, &expiry)
	if err != nil {
		return nil, err
	}
	//parse the string into time.Time
	//The format is 2022-01-05 16:30:57.513884+01:00
	expiryT, err = time.Parse("2006-01-02 15:04:05.999999999-07:00", expiry)
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{AccessToken: access_token, RefreshToken: refresh_token, Expiry: expiryT}, nil
}

/*		tok.AccessToken = os.Getenv("SPOTIFY_TOKEN")
		tok.RefreshToken
		tok.Expiry*/
func (p *PartyDB) SaveToken(tok *oauth2.Token) (int64, error) {
	conn := p.db
	//prepare insert statement
	stmt, err := conn.Prepare("INSERT INTO oath (access_token, refresh_token, expiry) VALUES (?,?,?)")

	if err != nil {
		return -1, err
	}
	defer stmt.Close()
	//execute the statement
	res, err := stmt.Exec(tok.AccessToken, tok.RefreshToken, tok.Expiry)
	//get the id of the oath token
	id, err2 := res.LastInsertId()

	if err != nil || err2 != nil {
		return -1, err
	}
	return id, nil
}

func generateInvitecode(n int) string {
	//generate a 4 character random string
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)

}
