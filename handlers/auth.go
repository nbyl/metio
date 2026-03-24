package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"slices"
	"time"

	"github.com/gorilla/sessions"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/firebase"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

const sessionName = "metio"
const userKey = "user"

const oauthGoogleUrlAPI = "https://www.googleapis.com/oauth2/v2/userinfo?access_token="

var store *sessions.CookieStore

func getSessionStore() *sessions.CookieStore {
	if store == nil {
		store = sessions.NewCookieStore([]byte(viper.GetString("SESSION_KEY")))
	}
	return store
}

var googleOauthConfig *oauth2.Config

func getGoogleOauthConfig() *oauth2.Config {
	if googleOauthConfig == nil {
		baseUrl := viper.GetString("BASE_URL")
		redirectUrl := fmt.Sprintf("%s/auth/callback", baseUrl)

		googleOauthConfig = &oauth2.Config{
			RedirectURL: redirectUrl,
			ClientID:    viper.GetString("GOOGLE_CLIENT_ID"),

			ClientSecret: viper.GetString("GOOGLE_CLIENT_SECRET"),
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.profile",
				"https://www.googleapis.com/auth/userinfo.email"},
			Endpoint: google.Endpoint,
		}
	}
	return googleOauthConfig
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	oauthState := generateStateOauthCookie(w)
	log.Println(r.URL.Path)
	log.Println(r.Host)
	log.Println(r.URL.Scheme)

	u := getGoogleOauthConfig().AuthCodeURL(oauthState)
	http.Redirect(w, r, u, http.StatusTemporaryRedirect)
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	// Read oauthState from Cookie
	oauthState, _ := r.Cookie("oauthstate")

	if r.FormValue("state") != oauthState.Value {
		log.Println("invalid oauth google state")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	user, err := getUserDataFromGoogle(r.FormValue("code"))
	if err != nil {
		log.Println(err.Error())
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	if !isUserAllowed(user) {
		log.Printf("User %s is not allowed", user.Email)
		http.Redirect(w, r, "/", http.StatusForbidden)
		return
	}

	session, _ := getSessionStore().Get(r, sessionName)

	session.Values[userKey] = user.ID
	err = session.Save(r, w)
	if err != nil {
		log.Println("Error saving session:", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func isUserAuthenticated(r *http.Request) bool {
	session, _ := getSessionStore().Get(r, sessionName)
	return !session.IsNew && session.Values[userKey] != nil
}

func isUserAllowed(user *User) bool {
	allowed_users := viper.GetStringSlice("ALLOWED_USERS")
	return slices.Contains(allowed_users, user.Email)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isUserAuthenticated(r) {
			http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
			return
		}

		// User is authenticated, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}

func generateStateOauthCookie(w http.ResponseWriter) string {
	var expiration = time.Now().Add(20 * time.Minute)

	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	cookie := http.Cookie{Name: "oauthstate", Value: state, Expires: expiration}
	http.SetCookie(w, &cookie)

	return state
}

func getUserDataFromGoogle(code string) (*User, error) {
	// Use code to get token and get user info from Google.

	token, err := getGoogleOauthConfig().Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("code exchange wrong: %s", err.Error())
	}
	response, err := http.Get(oauthGoogleUrlAPI + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed read response: %s", err.Error())
	}

	var user User
	err = json.Unmarshal(contents, &user)
	if err != nil {
		return nil, fmt.Errorf("error parsing response: %s", err.Error())
	}

	return &user, nil
}

// firebaseTokenHandler generates a Firebase custom token for the authenticated user
// This allows the frontend to authenticate with Firebase using the existing session
func firebaseTokenHandler(w http.ResponseWriter, r *http.Request) {
	session, err := getSessionStore().Get(r, sessionName)
	if err != nil {
		log.Printf("Error getting session: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	userID, ok := session.Values[userKey].(string)
	if !ok || userID == "" {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	token, err := firebase.CreateCustomToken(r.Context(), userID)
	if err != nil {
		log.Printf("Error creating Firebase token: %v", err)
		http.Error(w, "Failed to create Firebase token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}
