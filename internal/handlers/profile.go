package handlers

import (
	"context"
	"net/http"
	"thoughts/internal/db"
	"thoughts/internal/models"

	"github.com/gin-gonic/gin"
)

func GetFeed(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	rows, err := db.Pool.Query(context.Background(), `
		SELECT DISTINCT ON (t.id)
			t.id, t.author_id, u_a.username, t.subject_id, u_s.username,
			`+thoughtResolveCols("$1")+`
		FROM thoughts t
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

	var thoughts []models.ResolvedThought
	for rows.Next() {
		var rt models.ResolvedThought
		rows.Scan(&rt.ThoughtID, &rt.AuthorID, &rt.AuthorName, &rt.SubjectID, &rt.SubjectName,
			&rt.Content, &rt.IsDirect, &rt.CanSendCurious, &rt.CuriousPending)
		if rt.Content != "" {
			thoughts = append(thoughts, rt)
		}
	}

	c.HTML(http.StatusOK, "feed.html", gin.H{"User": user, "Thoughts": thoughts})
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
	thoughtRows, _ := db.Pool.Query(context.Background(), `
		SELECT t.id, t.author_id, u_a.username, t.subject_id, u_s.username,
			CASE WHEN t.author_id = $2 THEN
				COALESCE(
					(SELECT content FROM thought_versions WHERE thought_id = t.id AND audience_id = t.subject_id),
					(SELECT content FROM thought_versions WHERE thought_id = t.id AND audience_id IS NULL)
				)
			ELSE `+thoughtResolveContent("$2")+` END AS content,
			(t.author_id = $2 OR `+thoughtIsDirect("$2")+`) AS is_direct,
			(t.author_id <> $2 AND `+thoughtCanSendCurious("$2")+`) AS can_send_curious,
			`+thoughtCuriousPending("$2")+` AS curious_pending
		FROM thoughts t
		JOIN users u_a ON u_a.id = t.author_id
		JOIN users u_s ON u_s.id = t.subject_id
		WHERE t.author_id = $1
		ORDER BY t.updated_at DESC
	`, profile.ID, user.ID)
	defer thoughtRows.Close()

	var thoughts []models.ResolvedThought
	for thoughtRows.Next() {
		var rt models.ResolvedThought
		thoughtRows.Scan(&rt.ThoughtID, &rt.AuthorID, &rt.AuthorName, &rt.SubjectID, &rt.SubjectName,
			&rt.Content, &rt.IsDirect, &rt.CanSendCurious, &rt.CuriousPending)
		if rt.Content != "" {
			thoughts = append(thoughts, rt)
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
		"Thoughts":       thoughts,
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
