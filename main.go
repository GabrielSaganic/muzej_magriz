// main.go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type Link struct {
	ID                int    `json:"id"`
	TargetGalleryCode string `json:"target_gallery_code"`
	NameHR            string `json:"name_hr"`
	NameEN            string `json:"name_en"`
	NameIT            string `json:"name_it"`
	NameDE            string `json:"name_de"`
}

type MediaItem struct {
	ID           int    `json:"id"`
	Type         string `json:"type"` // "image" or "video"
	GalleryCode  string `json:"gallery_code"`
	URL          string `json:"url"`           // image_url or video_url
	ThumbnailURL string `json:"thumbnail_url"` // only for videos
	OrderIndex   int    `json:"order_index"`
	DescHR       string `json:"desc_hr"`
	DescEN       string `json:"desc_en"`
	DescIT       string `json:"desc_it"`
	DescDE       string `json:"desc_de"`
	LongDescHR   string `json:"long_desc_hr,omitempty"`
	LongDescEN   string `json:"long_desc_en,omitempty"`
	LongDescIT   string `json:"long_desc_it,omitempty"`
	LongDescDE   string `json:"long_desc_de,omitempty"`
	Links        []Link `json:"links,omitempty"`
}

var db *sql.DB

func main() {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"),
	)

	var err error
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil && db.Ping() == nil {
			break
		}
		log.Printf("Waiting for database... attempt %d", i+1)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/api/images", handleGetImages) // Keeping name for compatibility
	http.Handle("/", http.FileServer(http.Dir("./static")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleGetImages(w http.ResponseWriter, r *http.Request) {
	gallery := r.URL.Query().Get("gallery")
	if gallery == "" {
		gallery = "main"
	}

	var items []MediaItem

	// 1. Fetch images
	imgRows, err := db.Query(`
		SELECT id, gallery_code, image_url, order_index, 
		       desc_hr, desc_en, desc_it, desc_de, 
		       long_desc_hr, long_desc_en, long_desc_it, long_desc_de 
		FROM slike WHERE gallery_code = $1 
		ORDER BY order_index ASC`, gallery)

	if err == nil {
		defer imgRows.Close()
		for imgRows.Next() {
			var item MediaItem
			item.Type = "image"
			imgRows.Scan(&item.ID, &item.GalleryCode, &item.URL, &item.OrderIndex,
				&item.DescHR, &item.DescEN, &item.DescIT, &item.DescDE,
				&item.LongDescHR, &item.LongDescEN, &item.LongDescIT, &item.LongDescDE)

			// Fetch links for image
			linkRows, err := db.Query(`
				SELECT id, target_gallery_code, name_hr, name_en, name_it, name_de 
				FROM linkovi WHERE source_slika_id = $1`, item.ID)

			if err == nil {
				for linkRows.Next() {
					var l Link
					linkRows.Scan(&l.ID, &l.TargetGalleryCode, &l.NameHR, &l.NameEN, &l.NameIT, &l.NameDE)
					item.Links = append(item.Links, l)
				}
				linkRows.Close()
			}
			if item.Links == nil {
				item.Links = []Link{}
			}
			items = append(items, item)
		}
	}

	// 2. Fetch videos
	vidRows, err := db.Query(`
		SELECT id, gallery_code, video_url, thumbnail_url, order_index,
		       desc_hr, desc_en, desc_it, desc_de
		FROM videa WHERE gallery_code = $1
		ORDER BY order_index ASC`, gallery)

	if err == nil {
		defer vidRows.Close()
		for vidRows.Next() {
			var item MediaItem
			item.Type = "video"
			vidRows.Scan(&item.ID, &item.GalleryCode, &item.URL, &item.ThumbnailURL, &item.OrderIndex,
				&item.DescHR, &item.DescEN, &item.DescIT, &item.DescDE)

			item.Links = []Link{} // Videos don't have links in this schema yet
			items = append(items, item)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}
