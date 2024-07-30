package models

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"forum/Back-end/handlers"
)

type ModeratorRequest struct {
	UserID   int
	Username string
	Email    string
}

type Report struct {
	ID          int
	PostID      int
	ModeratorID int
	Reason      string
	CreatedAt   time.Time
}

type Feedback struct {
	ID        int
	ReportID  int
	Feedback  string
	CreatedAt time.Time
}

func HandleAdmin(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()
	tmpl, ok := tmplCache["panel"]
	if !ok {
		http.Error(w, "Could not load panel template", http.StatusInternalServerError)
		return
	}

	users, err := fetchUsers(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost {
		if r.FormValue("action") == "feedback" {
			reportIdStr := r.FormValue("reportId")
			reportId, err := strconv.Atoi(reportIdStr)
			if err != nil {
				http.Error(w, "Invalid report ID", http.StatusBadRequest)
				return
			}
			feedback := r.FormValue("feedback")
			if err := updateFeedback(db, reportId, feedback); err != nil {
				http.Error(w, "Failed to update feedback: "+err.Error(), http.StatusInternalServerError)
				log.Println("Failed to update feedback:", err)
				return
			}
			fmt.Fprintf(w, "Feedback added for report with ID %d", reportId)
			return
		}

		userIdStr := r.FormValue("userId")
		userId, err := strconv.Atoi(userIdStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		action := r.FormValue("action")
		if action == "accept" {
			if err := acceptModeratorRequest(db, userId); err != nil {
				http.Error(w, "Failed to accept moderator request: "+err.Error(), http.StatusInternalServerError)
				log.Println("Failed to accept moderator request:", err)
				return
			}
		} else if action == "reject" {
			if err := rejectModeratorRequest(db, userId); err != nil {
				http.Error(w, "Failed to reject moderator request: "+err.Error(), http.StatusInternalServerError)
				log.Println("Failed to reject moderator request:", err)
				return
			}
		} else if action == "delete" {
			if err := deletetable(db, userId); err != nil {
				http.Error(w, "Failed to delete user: "+err.Error(), http.StatusInternalServerError)
				log.Println("Failed to delete user:", err)
				return
			}
		}

		fmt.Fprintf(w, "Action %s completed for user with ID %d", action, userId)
	} else {
		requests, err := fetchModeratorRequests(db)
		if err != nil {
			http.Error(w, "Failed to fetch moderator requests: "+err.Error(), http.StatusInternalServerError)
			return
		}

		reports, err := fetchReports(db)
		if err != nil {
			http.Error(w, "Failed to fetch reports: "+err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			Users    []handlers.User
			Requests []ModeratorRequest
			Reports  []Report
		}{
			Requests: requests,
			Reports:  reports,
			Users:    users,
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func fetchModeratorRequests(db *sql.DB) ([]ModeratorRequest, error) {
	rows, err := db.Query(`
		SELECT mr.user_id, u.username, u.email
		FROM moderator_requests mr
		JOIN users u ON mr.user_id = u.id
		WHERE mr.status = 'pending'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []ModeratorRequest
	for rows.Next() {
		var request ModeratorRequest
		if err := rows.Scan(&request.UserID, &request.Username, &request.Email); err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, nil
}

func fetchReports(db *sql.DB) ([]Report, error) {
	rows, err := db.Query(`
		SELECT id, post_id, moderator_id, reason, created_at
		FROM reports
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []Report
	for rows.Next() {
		var report Report
		if err := rows.Scan(&report.ID, &report.PostID, &report.ModeratorID, &report.Reason, &report.CreatedAt); err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func updateFeedback(db *sql.DB, reportId int, feedback string) error {
	_, err := db.Exec(`
		INSERT INTO feedback (report_id, feedback, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(report_id) DO UPDATE SET feedback = excluded.feedback, created_at = excluded.created_at
	`, reportId, feedback)
	return err
}

func acceptModeratorRequest(db *sql.DB, userId int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE users SET is_moderator = true WHERE id = ?", userId)
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE moderator_requests SET status = 'accepted' WHERE user_id = ?", userId)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func rejectModeratorRequest(db *sql.DB, userId int) error {
	_, err := db.Exec("UPDATE moderator_requests SET status = 'rejected' WHERE user_id = ?", userId)
	return err
}

func deletetable(database *sql.DB, userId int) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	deleteStmt := "DELETE FROM users WHERE id = ?;"
	if _, err := tx.Exec(deleteStmt, userId); err != nil {
		return err
	}

	rowCount := 0
	countStmt := "SELECT COUNT(*) FROM users;"
	if err := tx.QueryRow(countStmt).Scan(&rowCount); err != nil {
		return err
	}

	if rowCount == 0 {
		resetAIStmt := "DELETE FROM SQLITE_SEQUENCE WHERE NAME = 'users';"
		if _, err := tx.Exec(resetAIStmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func HandleUserRoleChange(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		userID := r.FormValue("userId")
		isAdmin := r.FormValue("isAdmin") == "on"
		isModerator := r.FormValue("isModerator") == "on"

		db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer db.Close()

		_, err = db.Exec("UPDATE users SET is_admin = ?, is_moderator = ? WHERE id = ?", isAdmin, isModerator, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/panel", http.StatusSeeOther)
	}
}

func fetchUsers(db *sql.DB) ([]handlers.User, error) {
	rows, err := db.Query("SELECT id, username, email, is_admin, is_moderator FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []handlers.User
	for rows.Next() {
		var user handlers.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.IsModerator); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}
