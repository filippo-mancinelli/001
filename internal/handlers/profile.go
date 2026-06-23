package handlers

import (
	"context"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

func GetFeed(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	rows, err := db.Pool.Query(context.Background(), `
		SELECT DISTINCT ON (t.id)
			t.id, t.author_id, u_a.username, t.subject_id, u_s.username,
			`+colonneRisolte("$1")+`
		FROM pensieri t
		JOIN follows f ON f.following_id = t.author_id AND f.follower_id = $1
		JOIN users u_a ON u_a.id = t.author_id
		JOIN users u_s ON u_s.id = t.subject_id
		ORDER BY t.id, t.updated_at DESC
	`, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "feed.html", gin.H{"User": user})
		return
	}
	defer rows.Close()

	var pensieri []models.PensieroRisolto
	for rows.Next() {
		var rt models.PensieroRisolto
		rows.Scan(&rt.PensieroID, &rt.AuthorID, &rt.AuthorName, &rt.SubjectID, &rt.SubjectName,
			&rt.Content, &rt.IsDirect, &rt.CanSendCurious, &rt.CuriousPending, &rt.CommentCount, &rt.DnaCount, &rt.DnaDone, &rt.CreatedAt)
		if rt.Content != "" {
			pensieri = append(pensieri, rt)
		}
	}

	// pensieri pubblici recenti da utenti non seguiti (sezione scoperta)
	var pubblici []models.PensieroRisolto
	rows2, err2 := db.Pool.Query(context.Background(), `
		SELECT t.id, t.author_id, u_a.username, t.subject_id, u_s.username,
			`+colonneRisolte("$1")+`
		FROM pensieri t
		JOIN users u_a ON u_a.id = t.author_id
		JOIN users u_s ON u_s.id = t.subject_id
		WHERE t.author_id <> $1
		  AND NOT EXISTS (SELECT 1 FROM follows WHERE follower_id = $1 AND following_id = t.author_id)
		  AND EXISTS (
		    SELECT 1 FROM versioni_pensiero vp
		    WHERE vp.pensiero_id = t.id AND vp.audience_id IS NULL
		  )
		ORDER BY t.updated_at DESC
		LIMIT 30
	`, user.ID)
	if err2 == nil {
		defer rows2.Close()
		for rows2.Next() {
			var rt models.PensieroRisolto
			rows2.Scan(&rt.PensieroID, &rt.AuthorID, &rt.AuthorName, &rt.SubjectID, &rt.SubjectName,
				&rt.Content, &rt.IsDirect, &rt.CanSendCurious, &rt.CuriousPending, &rt.CommentCount, &rt.DnaCount, &rt.DnaDone, &rt.CreatedAt)
			if rt.Content != "" {
				pubblici = append(pubblici, rt)
			}
		}
	}

	c.HTML(http.StatusOK, "feed.html", gin.H{"User": user, "Pensieri": pensieri, "Pubblici": pubblici})
}

func GetProfile(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	username := c.Param("username")

	var profile models.User
	row := db.Pool.QueryRow(context.Background(),
		`SELECT `+models.UserColumns("")+` FROM users WHERE username = $1`, username)
	if err := models.ScanUser(row, &profile); err != nil {
		c.String(http.StatusNotFound, "utente non trovato")
		return
	}

	isSelf := profile.ID == user.ID

	var isFollowing bool
	if !isSelf {
		db.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id=$1 AND following_id=$2)`,
			user.ID, profile.ID,
		).Scan(&isFollowing)
	}

	// rete di conoscenze del profilo: chi segue (following) e chi lo segue (followers)
	following := connections(c, `
		SELECT u.id, u.username, COALESCE(u.display_name,''), COALESCE(u.avatar_url,''), COALESCE(u.presence,'online')
		FROM follows f JOIN users u ON u.id = f.following_id WHERE f.follower_id = $1
		ORDER BY u.username`, profile.ID)
	followers := connections(c, `
		SELECT u.id, u.username, COALESCE(u.display_name,''), COALESCE(u.avatar_url,''), COALESCE(u.presence,'online')
		FROM follows f JOIN users u ON u.id = f.follower_id WHERE f.following_id = $1
		ORDER BY u.username`, profile.ID)

	// pensieri scritti dal profilo, risolti per il viewer corrente.
	// l'autore vede sempre tutto ciò che ha scritto (versione diretta inclusa).
	righe, _ := db.Pool.Query(context.Background(), `
		SELECT t.id, t.author_id, u_a.username, t.subject_id, u_s.username,
			CASE WHEN t.author_id = $2 THEN
				COALESCE(
					(SELECT content FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = t.subject_id),
					(SELECT content FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id IS NULL)
				)
			ELSE `+resolviContenuto("$2")+` END AS content,
			(t.author_id = $2 OR `+isDiretta("$2")+`) AS is_direct,
			(t.author_id <> $2 AND `+puoCurioso("$2")+`) AS can_send_curious,
			`+curiosoInAttesa("$2")+` AS curious_pending,
			`+contaCommenti()+` AS comment_count,
			`+contaDna()+` AS dna_count,
			`+dnaFatto("$2")+` AS dna_done,
			t.created_at AS created_at
		FROM pensieri t
		JOIN users u_a ON u_a.id = t.author_id
		JOIN users u_s ON u_s.id = t.subject_id
		WHERE t.author_id = $1
		ORDER BY t.updated_at DESC
	`, profile.ID, user.ID)
	defer righe.Close()

	var pensieri []models.PensieroRisolto
	for righe.Next() {
		var rt models.PensieroRisolto
		righe.Scan(&rt.PensieroID, &rt.AuthorID, &rt.AuthorName, &rt.SubjectID, &rt.SubjectName,
			&rt.Content, &rt.IsDirect, &rt.CanSendCurious, &rt.CuriousPending, &rt.CommentCount, &rt.DnaCount, &rt.DnaDone, &rt.CreatedAt)
		if rt.Content != "" {
			pensieri = append(pensieri, rt)
		}
	}

	c.HTML(http.StatusOK, "profile.html", gin.H{
		"User":           user,
		"Profile":        profile,
		"IsSelf":         isSelf,
		"IsFollowing":    isFollowing,
		"Following":      following,
		"Followers":      followers,
		"FollowingCount": len(following),
		"FollowersCount": len(followers),
		"Pensieri":       pensieri,
	})
}

// connections esegue una query che restituisce
// (id, username, display_name, avatar_url, presence) e la mappa su []models.User.
func connections(c *gin.Context, query string, args ...any) []models.User {
	rows, err := db.Pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		if rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Presence) == nil {
			users = append(users, u)
		}
	}
	return users
}
