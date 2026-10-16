package recall

import "time"

// DueWordItem represents a word ready for spaced repetition review.
type DueWordItem struct {
	ID         string    `json:"id"`
	Term       string    `json:"term"`
	Definition string    `json:"definition"`
	Interval   int       `json:"interval"`
	EaseFactor float32   `json:"ease_factor"`
	NextReview time.Time `json:"next_review"`
}

// DueWordsResponse contains the list of due words.
type DueWordsResponse struct {
	Words []DueWordItem `json:"words"`
	Count int           `json:"count"`
}

// ReviewSubmission represents the user's score/feedback for an SRS item.
type ReviewSubmission struct {
	WordID string `json:"word_id"`
	Rating int    `json:"rating"` // 0-5 rating scale
}
