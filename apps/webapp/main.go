package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

type Config struct {
	Port               string
	Customer           string
	ColorHexPrimary    string
	ColorHexSecondary  string
	ColorHexBackground string
	Location           string
	Platform           string
}

var config = &Config{}

func main() {
	mux := http.NewServeMux()
	mux.Handle("/public/", logging(public()))
	mux.Handle("/", logging(index()))

	port, found := os.LookupEnv("PORT")
	if !found {
		port = "8080"
	}
	config.Port = port

	customer, found := os.LookupEnv("CUSTOMER")
	if !found {
		customer = "You!"
	}
	config.Customer = customer

	colorHexPrimary, found := os.LookupEnv("COLOR_PRIMARY")
	if !found {
		colorHexPrimary = "#000000"
	}
	config.ColorHexPrimary = colorHexPrimary

	colorHexSecondary, found := os.LookupEnv("COLOR_SECONDARY")
	if !found {
		colorHexSecondary = "#5f5f61"
	}
	config.ColorHexSecondary = colorHexSecondary

	colorHexBackground, found := os.LookupEnv("COLOR_BACKGROUND")
	if !found {
		colorHexBackground = "#FFFFFF"
	}
	config.ColorHexBackground = colorHexBackground

	location, found := os.LookupEnv("LOCATION")
	if !found {
		location = "a Google Cloud region"
	}
	config.Location = location

	platform, found := os.LookupEnv("PLATFORM")
	if !found {
		config.Platform = "Google Cloud Platform"
	}
	config.Platform = platform

	addr := fmt.Sprintf(":%s", config.Port)
	server := http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	log.Println("main: running simple server on port", config.Port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("main: couldn't start simple server: %v\n", err)
	}
}

// logging is middleware for wrapping any handler we want to track response
// times for and to see what resources are requested.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		req := fmt.Sprintf("%s %s", r.Method, r.URL)
		log.Println(req)
		next.ServeHTTP(w, r)
		log.Println(req, "completed in", time.Now().Sub(start))
	})
}

// templates references the specified templates and caches the parsed results
// to help speed up response times.
var templates = template.Must(template.ParseFiles("./templates/base.html", "./templates/body.html"))

// index is the handler responsible for rending the index page for the site.
func index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := struct {
			Title              template.HTML
			Customer           string
			Slogan             string
			ColorHexPrimary    string
			ColorHexSecondary  string
			ColorHexBackground string
			Location           string
			Platform           string
		}{
			Title:              template.HTML(fmt.Sprintf("%s - Can Scale", config.Customer)),
			Customer:           config.Customer,
			Slogan:             "can scale, with Google Cloud & Serverless Solutions.",
			Location:           config.Location,
			Platform:           config.Platform,
			ColorHexPrimary:    config.ColorHexPrimary,
			ColorHexSecondary:  config.ColorHexSecondary,
			ColorHexBackground: config.ColorHexBackground,
		}
		err := templates.ExecuteTemplate(w, "base", &b)
		if err != nil {
			http.Error(w, fmt.Sprintf("index: couldn't parse template: %v", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

// public serves static assets such as CSS and JavaScript to clients.
func public() http.Handler {
	return http.StripPrefix("/public/", http.FileServer(http.Dir("./public")))
}
