package models

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"forum/Back-end/handlers"
)

type Notification struct {
	ID        int
	UserID    int
	Message   string
	CreatedAt time.Time
	Read      bool
}

func CreateNotificationTable(db *sql.DB) {
	query := `
	CREATE TABLE IF NOT EXISTS notifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		message TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		read BOOLEAN DEFAULT FALSE,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Notification table creation failed: %s", err)
	}
}

func AddNotification(db *sql.DB, userID int, message string) error {
	query := `INSERT INTO notifications (user_id, message) VALUES (?, ?)`
	_, err := db.Exec(query, userID, message)
	return err
}

func GetNotifications(db *sql.DB, userID int) ([]Notification, error) {
	query := `SELECT id, user_id, message, created_at, read FROM notifications WHERE user_id = ? ORDER BY created_at DESC`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := []Notification{}
	for rows.Next() {
		var notification Notification
		err := rows.Scan(&notification.ID, &notification.UserID, &notification.Message, &notification.CreatedAt, &notification.Read)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return notifications, nil
}

func MarkNotificationAsRead(db *sql.DB, notificationID int) error {
	query := `UPDATE notifications SET read = TRUE WHERE id = ?`
	_, err := db.Exec(query, notificationID)
	return err
}

func GetAllPosts(db *sql.DB) ([]handlers.Post, error) {
	query := `
	SELECT p.id, p.user_id, u.username, p.title, p.content, p.image, p.category, c.name as category_name, p.created_at, p.likes, p.dislikes
	FROM posts p
	JOIN users u ON p.user_id = u.id
	JOIN categories c ON p.category = c.id
	ORDER BY p.created_at DESC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []handlers.Post{}
	for rows.Next() {
		var post handlers.Post
		err := rows.Scan(&post.ID, &post.UserID, &post.Username, &post.Title, &post.Content, &post.Image, &post.Category, &post.CategoryName, &post.CreatedAt, &post.Likes, &post.Dislikes)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func DeletePost(db *sql.DB, postID int) error {
	query := `DELETE FROM posts WHERE id = ?`
	_, err := db.Exec(query, postID)
	return err
}

func ReportPost(db *sql.DB, postID int, reporterID int, adminID int) error {
	// Get the post details
	query := `SELECT title, content FROM posts WHERE id = ?`
	row := db.QueryRow(query, postID)

	var title, content string
	err := row.Scan(&title, &content)
	if err != nil {
		return err
	}

	// Create a notification for the admin
	message := fmt.Sprintf("Reported Post ID: %d\nTitle: %s\nContent: %s\nReported by User ID: %d", postID, title, content, reporterID)
	return AddNotification(db, adminID, message)
}
