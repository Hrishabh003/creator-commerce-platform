package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Hrishabh003/creator-commerce-platform/backend/internal/ai"
	"github.com/Hrishabh003/creator-commerce-platform/backend/internal/db"
	"github.com/Hrishabh003/creator-commerce-platform/backend/internal/graph"
	"github.com/Hrishabh003/creator-commerce-platform/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// cors allows the local web and Expo dev servers to call the API.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "content-type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/commerce?sslmode=disable"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	st := &store.Store{Pool: pool}
	var drafter ai.Drafter = ai.TemplateDrafter{}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		drafter = ai.FallbackDrafter{
			Primary: ai.NewClaudeDrafter(key, env("ANTHROPIC_MODEL", "claude-sonnet-5-5")),
			Backup:  ai.TemplateDrafter{},
		}
		log.Println("drafting with Claude (template fallback enabled)")
	} else {
		log.Println("ANTHROPIC_API_KEY not set: using template drafter")
	}

	schema, err := graph.NewSchema(st, &ai.Pipeline{Retriever: st, Drafter: drafter})
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/graphql", cors(graph.Handler(schema)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	srv := &http.Server{Addr: ":" + env("PORT", "8080"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
