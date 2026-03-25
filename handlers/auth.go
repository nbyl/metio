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
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/spf13/viper"
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
const emailKey = "email"

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
		baseUrl := strings.TrimSuffix(viper.GetString("BASE_URL"), "/")
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
	oauthState, err := r.Cookie("oauthstate")
	if err != nil || oauthState == nil {
		log.Println("oauth state cookie not found")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

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
	session.Values[emailKey] = user.Email
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

// apiAuthMiddleware returns 401 JSON for unauthenticated API requests
// instead of redirecting to login page (used for JSON API endpoints)
func apiAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isUserAuthenticated(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
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

// AuthMeResponse represents the /api/auth/me response
type AuthMeResponse struct {
	Authenticated bool   `json:"authenticated"`
	Email         string `json:"email,omitempty"`
}

// meHandler returns the current user's authentication status
// Returns 200 with user data if authenticated, 401 if not
func meHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	session, err := getSessionStore().Get(r, sessionName)
	if err != nil || session.IsNew || session.Values[userKey] == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(AuthMeResponse{Authenticated: false})
		return
	}

	email, _ := session.Values[emailKey].(string)
	json.NewEncoder(w).Encode(AuthMeResponse{
		Authenticated: true,
		Email:         email,
	})
}
