package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

type graphNode struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Presence string `json:"presence"`
	IsSelf   bool   `json:"is_self"`
}

type graphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type graphData struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

func GetNetwork(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	nodeRows, err := db.Pool.Query(context.Background(), `
		WITH network AS (
			SELECT $1::uuid AS id
			UNION
			SELECT following_id FROM follows WHERE follower_id = $1
			UNION
			SELECT follower_id  FROM follows WHERE following_id = $1
		)
		SELECT u.id, u.username, `+models.PresenceExpr("u")+`
		FROM network n
		JOIN users u ON u.id = n.id
		ORDER BY u.username
	`, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "graph.html", gin.H{"User": user, "GraphJSON": template.JS("null")})
		return
	}
	defer nodeRows.Close()

	var nodes []graphNode
	for nodeRows.Next() {
		var n graphNode
		if nodeRows.Scan(&n.ID, &n.Username, &n.Presence) == nil {
			n.IsSelf = (n.ID == user.ID)
			nodes = append(nodes, n)
		}
	}

	edgeRows, err := db.Pool.Query(context.Background(), `
		WITH network AS (
			SELECT $1::uuid AS id
			UNION
			SELECT following_id FROM follows WHERE follower_id = $1
			UNION
			SELECT follower_id  FROM follows WHERE following_id = $1
		)
		SELECT f.follower_id, f.following_id
		FROM follows f
		WHERE f.follower_id  IN (SELECT id FROM network)
		  AND f.following_id IN (SELECT id FROM network)
	`, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "graph.html", gin.H{"User": user, "GraphJSON": template.JS("null")})
		return
	}
	defer edgeRows.Close()

	var edges []graphEdge
	for edgeRows.Next() {
		var e graphEdge
		edgeRows.Scan(&e.From, &e.To)
		edges = append(edges, e)
	}

	raw, _ := json.Marshal(graphData{Nodes: nodes, Edges: edges})
	c.HTML(http.StatusOK, "graph.html", gin.H{
		"User":      user,
		"GraphJSON": template.JS(raw),
	})
}
