package frontend

//go:generate go tool templ generate

//go:generate npx tailwindcss -c ./tailwind.config.js -i ./static/globals.css -o ./static/styles.css --minify
