package handlers

import (
	"net/http"
	"os"

	"github.com/dchest/uniuri"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func getGoogleOauthConfig() *oauth2.Config {
	return &oauth2.Config{
		RedirectURL:  "http://localhost:3000/oauth/callback",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint: google.Endpoint,
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	oauthStateString := uniuri.New()
	url := getGoogleOauthConfig().AuthCodeURL(oauthStateString)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {

}
