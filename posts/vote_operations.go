package posts

import (
	"fmt"
)

func (r *PostRepository) AddVote(postID, username, action string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	var exists bool
	err = tx.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM post_votes
            WHERE post_id = $1 AND username = $2
        )`, postID, username).Scan(&exists)

	if err != nil {
		tx.Rollback()
		return err
	}

	if exists {
		tx.Rollback()
		return fmt.Errorf("пользователь уже голосовал за этот пост")
	}

	var column string
	if action == "like" {
		column = "likes"
	} else if action == "dislike" {
		column = "dislikes"
	} else {
		tx.Rollback()
		return fmt.Errorf("неверное действие")
	}

	_, err = tx.Exec(`
        UPDATE posts
        SET `+column+` = `+column+` + 1
        WHERE id = $1`, postID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`
        INSERT INTO post_votes (post_id, username, vote_type)
        VALUES ($1, $2, $3)`,
		postID, username, action,
	)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
