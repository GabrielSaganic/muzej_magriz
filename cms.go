package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"golang.org/x/crypto/bcrypt"
)

const sessionDuration = 24 * time.Hour

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func cmsAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("cms_session")
		if err != nil {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var userID int
		var expiresAt time.Time
		err = db.QueryRow("SELECT user_id, expires_at FROM cms_sessions WHERE token = $1", cookie.Value).Scan(&userID, &expiresAt)
		if err != nil || time.Now().After(expiresAt) {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// extractPublicID extracts the Cloudinary public_id from a secure URL.
// e.g. https://res.cloudinary.com/xxx/image/upload/v123/muzej_magriz/file.jpg -> muzej_magriz/file
func extractPublicID(url string) string {
	idx := strings.Index(url, "/upload/")
	if idx == -1 {
		return ""
	}
	// after "/upload/" there's a version like "v1234567890/" then the public_id.ext
	after := url[idx+len("/upload/"):]
	// skip the version segment
	parts := strings.SplitN(after, "/", 2)
	if len(parts) < 2 {
		return ""
	}
	withExt := parts[1]
	// remove file extension
	ext := path.Ext(withExt)
	return strings.TrimSuffix(withExt, ext)
}

// destroyCloudinaryAsset deletes a single asset from Cloudinary by URL.
func destroyCloudinaryAsset(assetURL string, resourceType string) {
	if cld == nil || assetURL == "" {
		return
	}
	publicID := extractPublicID(assetURL)
	if publicID == "" {
		return
	}
	invalidate := true
	_, err := cld.Upload.Destroy(context.Background(), uploader.DestroyParams{
		PublicID:     publicID,
		ResourceType: resourceType,
		Invalidate:   &invalidate,
	})
	if err != nil {
		log.Printf("Warning: failed to delete from Cloudinary (public_id=%s): %v", publicID, err)
	} else {
		log.Printf("Deleted from Cloudinary: %s", publicID)
	}
}

func ensureCMSTables() {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cms_users (
			id SERIAL PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Printf("Warning: failed to create cms_users table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cms_sessions (
			id SERIAL PRIMARY KEY,
			user_id INT REFERENCES cms_users(id) ON DELETE CASCADE,
			token TEXT UNIQUE NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Printf("Warning: failed to create cms_sessions table: %v", err)
	}

	// Add long_desc columns to videa if they don't exist
	db.Exec("ALTER TABLE videa ADD COLUMN IF NOT EXISTS long_desc_hr TEXT")
	db.Exec("ALTER TABLE videa ADD COLUMN IF NOT EXISTS long_desc_en TEXT")
	db.Exec("ALTER TABLE videa ADD COLUMN IF NOT EXISTS long_desc_it TEXT")
	db.Exec("ALTER TABLE videa ADD COLUMN IF NOT EXISTS long_desc_de TEXT")
}

func seedAdminUser() {
	email := os.Getenv("CMS_ADMIN_EMAIL")
	password := os.Getenv("CMS_ADMIN_PASSWORD")
	if email == "" || password == "" {
		log.Println("CMS_ADMIN_EMAIL or CMS_ADMIN_PASSWORD not set, skipping admin seed")
		return
	}

	var exists bool
	db.QueryRow("SELECT EXISTS(SELECT 1 FROM cms_users WHERE email = $1)", email).Scan(&exists)
	if exists {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Failed to hash admin password: %v", err)
			return
		}
		db.Exec("UPDATE cms_users SET password_hash = $1 WHERE email = $2", string(hash), email)
		log.Printf("Admin user password updated: %s", email)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash admin password: %v", err)
		return
	}

	_, err = db.Exec("INSERT INTO cms_users (email, password_hash) VALUES ($1, $2)", email, string(hash))
	if err != nil {
		log.Printf("Failed to seed admin user: %v", err)
		return
	}
	log.Printf("Admin user seeded: %s", email)
}

// --- Auth handlers ---

func handleCMSLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var userID int
	var hash string
	err := db.QueryRow("SELECT id, password_hash FROM cms_users WHERE email = $1", req.Email).Scan(&userID, &hash)
	if err != nil {
		jsonError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		jsonError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Clean old sessions for this user
	db.Exec("DELETE FROM cms_sessions WHERE user_id = $1", userID)

	token := generateToken()
	expiresAt := time.Now().Add(sessionDuration)

	_, err = db.Exec("INSERT INTO cms_sessions (user_id, token, expires_at) VALUES ($1, $2, $3)", userID, token, expiresAt)
	if err != nil {
		jsonError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "cms_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Expires:  expiresAt,
		SameSite: http.SameSiteLaxMode,
	})

	jsonResponse(w, map[string]string{"status": "ok"})
}

func handleCMSLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("cms_session")
	if err == nil {
		db.Exec("DELETE FROM cms_sessions WHERE token = $1", cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "cms_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	jsonResponse(w, map[string]string{"status": "ok"})
}

func handleCMSMe(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("cms_session")
	if err != nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var email string
	var expiresAt time.Time
	err = db.QueryRow(`
		SELECT u.email, s.expires_at
		FROM cms_sessions s
		JOIN cms_users u ON u.id = s.user_id
		WHERE s.token = $1`, cookie.Value).Scan(&email, &expiresAt)
	if err != nil || time.Now().After(expiresAt) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	jsonResponse(w, map[string]string{"email": email})
}

// --- Image handlers ---

func handleCMSImages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		gallery := r.URL.Query().Get("gallery")
		var query string
		var args []interface{}

		if gallery != "" {
			query = `SELECT id, gallery_code, image_url, order_index,
				COALESCE(desc_hr,''), COALESCE(desc_en,''), COALESCE(desc_it,''), COALESCE(desc_de,''),
				COALESCE(long_desc_hr,''), COALESCE(long_desc_en,''), COALESCE(long_desc_it,''), COALESCE(long_desc_de,'')
				FROM slike WHERE gallery_code = $1 ORDER BY order_index ASC`
			args = []interface{}{gallery}
		} else {
			query = `SELECT id, gallery_code, image_url, order_index,
				COALESCE(desc_hr,''), COALESCE(desc_en,''), COALESCE(desc_it,''), COALESCE(desc_de,''),
				COALESCE(long_desc_hr,''), COALESCE(long_desc_en,''), COALESCE(long_desc_it,''), COALESCE(long_desc_de,'')
				FROM slike ORDER BY gallery_code, order_index ASC`
		}

		rows, err := db.Query(query, args...)
		if err != nil {
			jsonError(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var items []MediaItem
		for rows.Next() {
			var item MediaItem
			item.Type = "image"
			rows.Scan(&item.ID, &item.GalleryCode, &item.URL, &item.OrderIndex,
				&item.DescHR, &item.DescEN, &item.DescIT, &item.DescDE,
				&item.LongDescHR, &item.LongDescEN, &item.LongDescIT, &item.LongDescDE)
			items = append(items, item)
		}
		if items == nil {
			items = []MediaItem{}
		}
		jsonResponse(w, items)

	case http.MethodPost:
		if err := r.ParseMultipartForm(50 << 20); err != nil {
			jsonError(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			jsonError(w, "File is required", http.StatusBadRequest)
			return
		}
		defer file.Close()

		if cld == nil {
			jsonError(w, "Cloudinary not configured", http.StatusInternalServerError)
			return
		}

		resp, err := cld.Upload.Upload(r.Context(), file, uploader.UploadParams{
			Folder: "muzej_magriz",
		})
		if err != nil {
			jsonError(w, "Upload failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		gallery := r.FormValue("gallery_code")
		if gallery == "" {
			gallery = "main"
		}
		orderIndex, _ := strconv.Atoi(r.FormValue("order_index"))

		var id int
		err = db.QueryRow(`
			INSERT INTO slike (gallery_code, image_url, order_index, desc_hr, desc_en, desc_it, desc_de,
				long_desc_hr, long_desc_en, long_desc_it, long_desc_de)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
			gallery, resp.SecureURL, orderIndex,
			r.FormValue("desc_hr"), r.FormValue("desc_en"), r.FormValue("desc_it"), r.FormValue("desc_de"),
			r.FormValue("long_desc_hr"), r.FormValue("long_desc_en"), r.FormValue("long_desc_it"), r.FormValue("long_desc_de"),
		).Scan(&id)

		if err != nil {
			jsonError(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		jsonResponse(w, map[string]interface{}{"id": id, "url": resp.SecureURL})

	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleCMSImage(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		jsonError(w, "Invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req struct {
			GalleryCode string `json:"gallery_code"`
			ImageURL    string `json:"image_url"`
			OrderIndex  int    `json:"order_index"`
			DescHR      string `json:"desc_hr"`
			DescEN      string `json:"desc_en"`
			DescIT      string `json:"desc_it"`
			DescDE      string `json:"desc_de"`
			LongDescHR  string `json:"long_desc_hr"`
			LongDescEN  string `json:"long_desc_en"`
			LongDescIT  string `json:"long_desc_it"`
			LongDescDE  string `json:"long_desc_de"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		_, err = db.Exec(`
			UPDATE slike SET gallery_code=$1, image_url=$2, order_index=$3,
			desc_hr=$4, desc_en=$5, desc_it=$6, desc_de=$7,
			long_desc_hr=$8, long_desc_en=$9, long_desc_it=$10, long_desc_de=$11
			WHERE id=$12`,
			req.GalleryCode, req.ImageURL, req.OrderIndex,
			req.DescHR, req.DescEN, req.DescIT, req.DescDE,
			req.LongDescHR, req.LongDescEN, req.LongDescIT, req.LongDescDE,
			id)
		if err != nil {
			jsonError(w, "Database error", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]string{"status": "ok"})

	case http.MethodDelete:
		// Fetch image URL before deleting so we can remove it from Cloudinary
		var imageURL string
		db.QueryRow("SELECT COALESCE(image_url,'') FROM slike WHERE id = $1", id).Scan(&imageURL)

		_, err = db.Exec("DELETE FROM slike WHERE id = $1", id)
		if err != nil {
			jsonError(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Delete from Cloudinary in the background
		go destroyCloudinaryAsset(imageURL, "image")

		jsonResponse(w, map[string]string{"status": "ok"})

	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Video handlers ---

func handleCMSVideos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		gallery := r.URL.Query().Get("gallery")
		var query string
		var args []interface{}

		if gallery != "" {
			query = `SELECT id, gallery_code, video_url, COALESCE(thumbnail_url,''), order_index,
				COALESCE(desc_hr,''), COALESCE(desc_en,''), COALESCE(desc_it,''), COALESCE(desc_de,''),
				COALESCE(long_desc_hr,''), COALESCE(long_desc_en,''), COALESCE(long_desc_it,''), COALESCE(long_desc_de,'')
				FROM videa WHERE gallery_code = $1 ORDER BY order_index ASC`
			args = []interface{}{gallery}
		} else {
			query = `SELECT id, gallery_code, video_url, COALESCE(thumbnail_url,''), order_index,
				COALESCE(desc_hr,''), COALESCE(desc_en,''), COALESCE(desc_it,''), COALESCE(desc_de,''),
				COALESCE(long_desc_hr,''), COALESCE(long_desc_en,''), COALESCE(long_desc_it,''), COALESCE(long_desc_de,'')
				FROM videa ORDER BY gallery_code, order_index ASC`
		}

		rows, err := db.Query(query, args...)
		if err != nil {
			jsonError(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var items []MediaItem
		for rows.Next() {
			var item MediaItem
			item.Type = "video"
			rows.Scan(&item.ID, &item.GalleryCode, &item.URL, &item.ThumbnailURL, &item.OrderIndex,
				&item.DescHR, &item.DescEN, &item.DescIT, &item.DescDE,
				&item.LongDescHR, &item.LongDescEN, &item.LongDescIT, &item.LongDescDE)
			items = append(items, item)
		}
		if items == nil {
			items = []MediaItem{}
		}
		jsonResponse(w, items)

	case http.MethodPost:
		if err := r.ParseMultipartForm(200 << 20); err != nil {
			jsonError(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		videoURL := r.FormValue("video_url")
		thumbnailURL := r.FormValue("thumbnail_url")

		// Upload video file if provided
		file, _, err := r.FormFile("file")
		if err == nil {
			defer file.Close()
			if cld != nil {
				resp, err := cld.Upload.Upload(r.Context(), file, uploader.UploadParams{
					Folder:       "muzej_magriz",
					ResourceType: "video",
				})
				if err == nil {
					videoURL = resp.SecureURL
				} else {
					jsonError(w, "Video upload failed: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

		// Upload thumbnail if provided
		thumbFile, _, err := r.FormFile("thumbnail")
		if err == nil {
			defer thumbFile.Close()
			if cld != nil {
				resp, err := cld.Upload.Upload(r.Context(), thumbFile, uploader.UploadParams{
					Folder: "muzej_magriz",
				})
				if err == nil {
					thumbnailURL = resp.SecureURL
				}
			}
		}

		if videoURL == "" {
			jsonError(w, "Video URL or file is required", http.StatusBadRequest)
			return
		}

		gallery := r.FormValue("gallery_code")
		if gallery == "" {
			gallery = "main"
		}
		orderIndex, _ := strconv.Atoi(r.FormValue("order_index"))

		var id int
		err = db.QueryRow(`
			INSERT INTO videa (gallery_code, video_url, thumbnail_url, order_index,
				desc_hr, desc_en, desc_it, desc_de,
				long_desc_hr, long_desc_en, long_desc_it, long_desc_de)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id`,
			gallery, videoURL, thumbnailURL, orderIndex,
			r.FormValue("desc_hr"), r.FormValue("desc_en"), r.FormValue("desc_it"), r.FormValue("desc_de"),
			r.FormValue("long_desc_hr"), r.FormValue("long_desc_en"), r.FormValue("long_desc_it"), r.FormValue("long_desc_de"),
		).Scan(&id)

		if err != nil {
			jsonError(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		jsonResponse(w, map[string]interface{}{"id": id, "url": videoURL})

	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleCMSVideo(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		jsonError(w, "Invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req struct {
			GalleryCode  string `json:"gallery_code"`
			VideoURL     string `json:"video_url"`
			ThumbnailURL string `json:"thumbnail_url"`
			OrderIndex   int    `json:"order_index"`
			DescHR       string `json:"desc_hr"`
			DescEN       string `json:"desc_en"`
			DescIT       string `json:"desc_it"`
			DescDE       string `json:"desc_de"`
			LongDescHR   string `json:"long_desc_hr"`
			LongDescEN   string `json:"long_desc_en"`
			LongDescIT   string `json:"long_desc_it"`
			LongDescDE   string `json:"long_desc_de"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		_, err = db.Exec(`
			UPDATE videa SET gallery_code=$1, video_url=$2, thumbnail_url=$3, order_index=$4,
			desc_hr=$5, desc_en=$6, desc_it=$7, desc_de=$8,
			long_desc_hr=$9, long_desc_en=$10, long_desc_it=$11, long_desc_de=$12
			WHERE id=$13`,
			req.GalleryCode, req.VideoURL, req.ThumbnailURL, req.OrderIndex,
			req.DescHR, req.DescEN, req.DescIT, req.DescDE,
			req.LongDescHR, req.LongDescEN, req.LongDescIT, req.LongDescDE,
			id)
		if err != nil {
			jsonError(w, "Database error", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]string{"status": "ok"})

	case http.MethodDelete:
		// Fetch URLs before deleting so we can remove them from Cloudinary
		var videoURL, thumbnailURL string
		db.QueryRow("SELECT COALESCE(video_url,''), COALESCE(thumbnail_url,'') FROM videa WHERE id = $1", id).Scan(&videoURL, &thumbnailURL)

		_, err = db.Exec("DELETE FROM videa WHERE id = $1", id)
		if err != nil {
			jsonError(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Delete from Cloudinary in the background
		go destroyCloudinaryAsset(videoURL, "video")
		go destroyCloudinaryAsset(thumbnailURL, "image")

		jsonResponse(w, map[string]string{"status": "ok"})

	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Gallery handler ---

func handleCMSGalleries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query(`
		SELECT DISTINCT gallery_code FROM (
			SELECT gallery_code FROM slike
			UNION
			SELECT gallery_code FROM videa
			UNION
			SELECT target_gallery_code AS gallery_code FROM linkovi
		) combined ORDER BY gallery_code`)
	if err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var galleries []string
	for rows.Next() {
		var g string
		rows.Scan(&g)
		galleries = append(galleries, g)
	}
	if galleries == nil {
		galleries = []string{}
	}
	jsonResponse(w, galleries)
}

// --- Link handlers ---

type LinkWithSource struct {
	ID                int    `json:"id"`
	SourceImageID     int    `json:"source_slika_id"`
	TargetGalleryCode string `json:"target_gallery_code"`
	NameHR            string `json:"name_hr"`
	NameEN            string `json:"name_en"`
	NameIT            string `json:"name_it"`
	NameDE            string `json:"name_de"`
}

func handleCMSLinks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		imageID := r.URL.Query().Get("image_id")
		if imageID == "" {
			rows, err := db.Query(`SELECT id, source_slika_id, target_gallery_code,
				COALESCE(name_hr,''), COALESCE(name_en,''), COALESCE(name_it,''), COALESCE(name_de,'')
				FROM linkovi ORDER BY id`)
			if err != nil {
				jsonError(w, "Database error", http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			var links []LinkWithSource
			for rows.Next() {
				var l LinkWithSource
				rows.Scan(&l.ID, &l.SourceImageID, &l.TargetGalleryCode, &l.NameHR, &l.NameEN, &l.NameIT, &l.NameDE)
				links = append(links, l)
			}
			if links == nil {
				links = []LinkWithSource{}
			}
			jsonResponse(w, links)
		} else {
			imgID, _ := strconv.Atoi(imageID)
			rows, err := db.Query(`SELECT id, target_gallery_code,
				COALESCE(name_hr,''), COALESCE(name_en,''), COALESCE(name_it,''), COALESCE(name_de,'')
				FROM linkovi WHERE source_slika_id = $1`, imgID)
			if err != nil {
				jsonError(w, "Database error", http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			var links []Link
			for rows.Next() {
				var l Link
				rows.Scan(&l.ID, &l.TargetGalleryCode, &l.NameHR, &l.NameEN, &l.NameIT, &l.NameDE)
				links = append(links, l)
			}
			if links == nil {
				links = []Link{}
			}
			jsonResponse(w, links)
		}

	case http.MethodPost:
		var req struct {
			SourceImageID     int    `json:"source_slika_id"`
			TargetGalleryCode string `json:"target_gallery_code"`
			NameHR            string `json:"name_hr"`
			NameEN            string `json:"name_en"`
			NameIT            string `json:"name_it"`
			NameDE            string `json:"name_de"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		var id int
		err := db.QueryRow(`
			INSERT INTO linkovi (source_slika_id, target_gallery_code, name_hr, name_en, name_it, name_de)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			req.SourceImageID, req.TargetGalleryCode, req.NameHR, req.NameEN, req.NameIT, req.NameDE,
		).Scan(&id)
		if err != nil {
			jsonError(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]int{"id": id})

	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			jsonError(w, "Invalid id", http.StatusBadRequest)
			return
		}
		_, err = db.Exec("DELETE FROM linkovi WHERE id = $1", id)
		if err != nil {
			jsonError(w, "Database error", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]string{"status": "ok"})

	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Upload handler (generic, for replacing images/thumbnails) ---

func handleCMSUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if cld == nil {
		jsonError(w, "Cloudinary not configured", http.StatusInternalServerError)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	resourceType := r.FormValue("resource_type")
	if resourceType == "" {
		resourceType = "image"
	}

	resp, err := cld.Upload.Upload(r.Context(), file, uploader.UploadParams{
		Folder:       "muzej_magriz",
		ResourceType: resourceType,
	})
	if err != nil {
		jsonError(w, "Upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{
		"url":       resp.SecureURL,
		"public_id": resp.PublicID,
	})
}

// --- Route registration ---

func registerCMSRoutes() {
	http.HandleFunc("/api/cms/login", handleCMSLogin)
	http.HandleFunc("/api/cms/logout", handleCMSLogout)
	http.HandleFunc("/api/cms/me", cmsAuth(handleCMSMe))
	http.HandleFunc("/api/cms/images", cmsAuth(handleCMSImages))
	http.HandleFunc("/api/cms/image", cmsAuth(handleCMSImage))
	http.HandleFunc("/api/cms/videos", cmsAuth(handleCMSVideos))
	http.HandleFunc("/api/cms/video", cmsAuth(handleCMSVideo))
	http.HandleFunc("/api/cms/galleries", cmsAuth(handleCMSGalleries))
	http.HandleFunc("/api/cms/links", cmsAuth(handleCMSLinks))
	http.HandleFunc("/api/cms/upload", cmsAuth(handleCMSUpload))
}
