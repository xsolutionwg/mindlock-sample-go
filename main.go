package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

//go:embed head.html
var head string

type note struct {
	Text string `bson:"text" json:"text"`
}

func main() {
	password, err := os.ReadFile(os.Getenv("MONGODB_PASSWORD_FILE"))
	if err != nil {
		log.Fatal(err)
	}
	db := os.Getenv("MONGODB_DATABASE")
	uri := fmt.Sprintf("mongodb://%s:%s@%s:27017/%s?authSource=%s", os.Getenv("MONGODB_USER"),
		url.QueryEscape(strings.TrimSpace(string(password))), os.Getenv("MONGODB_HOST"), db, db)
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	notes := client.Database(db).Collection("notes")

	recent := func(ctx context.Context) ([]note, error) {
		opts := options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(50)
		cursor, err := notes.Find(ctx, bson.D{}, opts)
		if err != nil {
			return nil, err
		}
		out := []note{}
		return out, cursor.All(ctx, &out)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/notes", func(w http.ResponseWriter, r *http.Request) {
		list, err := recent(r.Context())
		if err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(list)
	})
	mux.HandleFunc("POST /notes", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		text := strings.TrimSpace(r.FormValue("text"))
		if len(text) > 200 {
			text = text[:200]
		}
		if text != "" {
			if _, err := notes.InsertOne(r.Context(), note{Text: text}); err != nil {
				http.Error(w, "database unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		list, err := recent(r.Context())
		if err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var b strings.Builder
		b.WriteString(head)
		for _, n := range list {
			b.WriteString("<li>" + html.EscapeString(n.Text) + "</li>")
		}
		b.WriteString("</ul>")
		w.Header().Set("content-type", "text/html; charset=utf-8")
		w.Write([]byte(b.String()))
	})

	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Println("listening on 8080")
	log.Fatal(server.ListenAndServe())
}
