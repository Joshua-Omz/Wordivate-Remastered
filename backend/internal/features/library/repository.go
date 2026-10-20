// Purpose: Persistence layer for atomic transactional commits, idempotency, and vocabulary library queries.
// Architecture: backend/internal/features/library/repository.go
package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/Joshua-Omz/wordivate/backend/internal/database/sqlc"
)

/// SessionRecord holds minimal session state needed for commit validation.
type SessionRecord struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	WordText      string
	HeadwordKey   string
	Status        string
	ContextNote   string
	SnapshotID    uuid.UUID
	PromptVersion string
	ModelID       string
	StartedAt     time.Time
	EndedAt       *time.Time
}

/// SnapshotRecord holds lexical snapshot data for commit official definition grounding.
type SnapshotRecord struct {
	ID          uuid.UUID
	HeadwordKey string
	Payload     []byte
}

/// IdempotencyData holds cached mutation state for duplicate request suppression.
type IdempotencyData struct {
	UserID      uuid.UUID
	Key         string
	RequestHash []byte
	Response    []byte
	CreatedAt   time.Time
}

/// CommitTxParams encapsulates all records to be inserted/updated within the atomic commit transaction.
type CommitTxParams struct {
	UserID             uuid.UUID
	SessionID          uuid.UUID
	WordText           string
	HeadwordKey        string
	SenseKey           string
	SenseLabel         string
	PartOfSpeech       string
	SnapshotID         uuid.UUID
	OfficialDefinition string
	PersonalDefinition string
	PersonalDefOrigin  PersonalDefOrigin
	ExampleSentence    string
	ContextualNotes    string
	UnderstandingFlag  UnderstandingFlag
	DiscrepancyNote    *string
	AssessmentVerdict  string
	AssessmentReason   string
	AssessmentRaw      map[string]interface{}
	PromptVersion      string
}

/// LibraryRepository defines the database contract for vocabulary entries, commits, and idempotency.
type LibraryRepository interface {
	GetSessionForCommit(ctx context.Context, sessionID, userID uuid.UUID) (*SessionRecord, error)
	GetSessionMessages(ctx context.Context, sessionID uuid.UUID) ([]HistoryMessage, error)
	GetSnapshotByID(ctx context.Context, snapshotID uuid.UUID) (*SnapshotRecord, error)
	UpdateSessionStatus(ctx context.Context, sessionID, userID uuid.UUID, status string) error
	GetIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (*IdempotencyData, error)
	CreateIdempotencyKey(ctx context.Context, userID uuid.UUID, key string, requestHash []byte) error
	SaveIdempotencyResponse(ctx context.Context, userID uuid.UUID, key string, response []byte) error
	CommitSessionTx(ctx context.Context, params CommitTxParams) (*VocabEntry, *CommitAssessment, error)
	GetEntryByID(ctx context.Context, id, userID uuid.UUID) (*VocabEntry, error)
	GetEntryRevisions(ctx context.Context, entryID uuid.UUID) ([]EntryRevision, error)
	ListEntries(ctx context.Context, userID uuid.UUID, filter ListEntriesFilter) ([]VocabEntry, error)
	SearchEntries(ctx context.Context, userID uuid.UUID, query string, limit, offset int32) ([]VocabEntry, error)
	UpdateEntryPersonalTx(ctx context.Context, userID, entryID uuid.UUID, req UpdateEntryRequest) (*VocabEntry, *EntryRevision, error)
	DeleteEntry(ctx context.Context, id, userID uuid.UUID) error
}

type sqlRepository struct {
	db *sql.DB
	q  *sqlc.Queries
}

/// NewRepository instantiates a concrete LibraryRepository backed by PostgreSQL.
func NewRepository(db *sql.DB) LibraryRepository {
	return &sqlRepository{
		db: db,
		q:  sqlc.New(db),
	}
}

func mapVocabEntry(e sqlc.VocabEntry) *VocabEntry {
	var label, pos, example, notes string
	if e.SenseLabel.Valid {
		label = e.SenseLabel.String
	}
	if e.PartOfSpeech.Valid {
		pos = e.PartOfSpeech.String
	}
	if e.ExampleSentence.Valid {
		example = e.ExampleSentence.String
	}
	if e.ContextualNotes.Valid {
		notes = e.ContextualNotes.String
	}
	var discrepancy *string
	if e.DiscrepancyNote.Valid {
		d := e.DiscrepancyNote.String
		discrepancy = &d
	}

	return &VocabEntry{
		ID:                 e.ID,
		UserID:             e.UserID,
		WordText:           e.WordText,
		HeadwordKey:        e.HeadwordKey,
		SenseKey:           e.SenseKey,
		SenseLabel:         label,
		PartOfSpeech:       pos,
		SnapshotID:         e.SnapshotID,
		OfficialDefinition: e.OfficialDefinition,
		PersonalDefinition: e.PersonalDefinition,
		PersonalDefOrigin:  PersonalDefOrigin(e.PersonalDefOrigin),
		ExampleSentence:    example,
		ContextualNotes:    notes,
		UnderstandingFlag:  UnderstandingFlag(e.UnderstandingFlag),
		DiscrepancyNote:    discrepancy,
		OriginSessionID:    e.OriginSessionID,
		CreatedAt:          e.CreatedAt,
		UpdatedAt:          e.UpdatedAt,
	}
}

func mapEntryRevision(r sqlc.EntryRevision) (*EntryRevision, error) {
	var changes map[string]interface{}
	if len(r.Changes) > 0 {
		if err := json.Unmarshal(r.Changes, &changes); err != nil {
			return nil, fmt.Errorf("decoding revision changes: %w", err)
		}
	}
	return &EntryRevision{
		ID:        r.ID,
		EntryID:   r.EntryID,
		RevNo:     r.RevNo,
		Changes:   changes,
		ChangedAt: r.ChangedAt,
	}, nil
}

func (r *sqlRepository) GetSessionForCommit(ctx context.Context, sessionID, userID uuid.UUID) (*SessionRecord, error) {
	s, err := r.q.GetSessionByID(ctx, sqlc.GetSessionByIDParams{
		ID:     sessionID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("querying session for commit: %w", err)
	}

	var note, model string
	if s.ContextNote.Valid {
		note = s.ContextNote.String
	}
	if s.ModelID.Valid {
		model = s.ModelID.String
	}
	var snapID uuid.UUID
	if s.SnapshotID.Valid {
		snapID = s.SnapshotID.UUID
	}
	var ended *time.Time
	if s.EndedAt.Valid {
		t := s.EndedAt.Time
		ended = &t
	}

	return &SessionRecord{
		ID:            s.ID,
		UserID:        s.UserID,
		WordText:      s.WordText,
		HeadwordKey:   s.HeadwordKey,
		Status:        s.Status,
		ContextNote:   note,
		SnapshotID:    snapID,
		PromptVersion: s.PromptVersion,
		ModelID:       model,
		StartedAt:     s.StartedAt,
		EndedAt:       ended,
	}, nil
}

func (r *sqlRepository) GetSessionMessages(ctx context.Context, sessionID uuid.UUID) ([]HistoryMessage, error) {
	msgs, err := r.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listing session messages: %w", err)
	}

	result := make([]HistoryMessage, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, HistoryMessage{
			Seq:       m.Seq,
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		})
	}
	return result, nil
}

func (r *sqlRepository) GetSnapshotByID(ctx context.Context, snapshotID uuid.UUID) (*SnapshotRecord, error) {
	snap, err := r.q.GetDefinitionSnapshotByID(ctx, snapshotID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("snapshot not found")
		}
		return nil, fmt.Errorf("getting definition snapshot: %w", err)
	}

	return &SnapshotRecord{
		ID:          snap.ID,
		HeadwordKey: snap.HeadwordKey,
		Payload:     snap.Payload,
	}, nil
}

func (r *sqlRepository) UpdateSessionStatus(ctx context.Context, sessionID, userID uuid.UUID, status string) error {
	_, err := r.q.UpdateSessionStatus(ctx, sqlc.UpdateSessionStatusParams{
		ID:     sessionID,
		Status: status,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("updating session status: %w", err)
	}
	return nil
}

func (r *sqlRepository) GetIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (*IdempotencyData, error) {
	k, err := r.q.GetIdempotencyKey(ctx, sqlc.GetIdempotencyKeyParams{
		UserID: userID,
		Key:    key,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Key does not exist
		}
		return nil, fmt.Errorf("getting idempotency key: %w", err)
	}

	var respBytes []byte
	if k.Response.Valid {
		respBytes = k.Response.RawMessage
	}

	return &IdempotencyData{
		UserID:      k.UserID,
		Key:         k.Key,
		RequestHash: k.RequestHash,
		Response:    respBytes,
		CreatedAt:   k.CreatedAt,
	}, nil
}

func (r *sqlRepository) CreateIdempotencyKey(ctx context.Context, userID uuid.UUID, key string, requestHash []byte) error {
	_, err := r.q.CreateIdempotencyKey(ctx, sqlc.CreateIdempotencyKeyParams{
		UserID:      userID,
		Key:         key,
		RequestHash: requestHash,
	})
	if err != nil {
		return fmt.Errorf("inserting idempotency key: %w", err)
	}
	return nil
}

func (r *sqlRepository) SaveIdempotencyResponse(ctx context.Context, userID uuid.UUID, key string, response []byte) error {
	nullRaw := pqtype.NullRawMessage{
		RawMessage: json.RawMessage(response),
		Valid:      len(response) > 0,
	}
	_, err := r.q.UpdateIdempotencyResponse(ctx, sqlc.UpdateIdempotencyResponseParams{
		UserID:   userID,
		Key:      key,
		Response: nullRaw,
	})
	if err != nil {
		return fmt.Errorf("saving idempotency response: %w", err)
	}
	return nil
}

/// CommitSessionTx executes the atomic multi-table commit transaction.
func (r *sqlRepository) CommitSessionTx(ctx context.Context, params CommitTxParams) (*VocabEntry, *CommitAssessment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("beginning commit transaction: %w", err)
	}
	// Defer rollback ensures clean abort if any step returns error before tx.Commit()
	defer tx.Rollback()

	qtx := r.q.WithTx(tx)

	// 1. Insert vocabulary entry into vocab_entries
	var discNote sql.NullString
	if params.DiscrepancyNote != nil {
		discNote = sql.NullString{String: *params.DiscrepancyNote, Valid: true}
	}

	entryRow, err := qtx.CreateVocabEntry(ctx, sqlc.CreateVocabEntryParams{
		UserID:             params.UserID,
		WordText:           params.WordText,
		HeadwordKey:        params.HeadwordKey,
		SenseKey:           params.SenseKey,
		SenseLabel:         sql.NullString{String: params.SenseLabel, Valid: params.SenseLabel != ""},
		PartOfSpeech:       sql.NullString{String: params.PartOfSpeech, Valid: params.PartOfSpeech != ""},
		SnapshotID:         params.SnapshotID,
		OfficialDefinition: params.OfficialDefinition,
		PersonalDefinition: params.PersonalDefinition,
		PersonalDefOrigin:  string(params.PersonalDefOrigin),
		ExampleSentence:    sql.NullString{String: params.ExampleSentence, Valid: params.ExampleSentence != ""},
		ContextualNotes:    sql.NullString{String: params.ContextualNotes, Valid: params.ContextualNotes != ""},
		UnderstandingFlag:  string(params.UnderstandingFlag),
		DiscrepancyNote:    discNote,
		OriginSessionID:    params.SessionID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("inserting vocab entry: %w", err)
	}

	// 2. Insert initial revision (rev 1) into entry_revisions (FR-034)
	rev1Changes := map[string]interface{}{
		"personal_definition": params.PersonalDefinition,
		"example_sentence":    params.ExampleSentence,
		"contextual_notes":    params.ContextualNotes,
		"understanding_flag":  params.UnderstandingFlag,
		"revision_type":       "initial_commit",
	}
	rev1JSON, err := json.Marshal(rev1Changes)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling revision 1 changes: %w", err)
	}

	_, err = qtx.CreateEntryRevision(ctx, sqlc.CreateEntryRevisionParams{
		EntryID: entryRow.ID,
		RevNo:   1,
		Changes: rev1JSON,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("inserting entry revision 1: %w", err)
	}

	// 3. Insert AI commit assessment into commit_assessments
	rawAssessmentJSON, err := json.Marshal(params.AssessmentRaw)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling assessment raw output: %w", err)
	}

	assessmentRow, err := qtx.CreateCommitAssessment(ctx, sqlc.CreateCommitAssessmentParams{
		SessionID:       params.SessionID,
		Verdict:         params.AssessmentVerdict,
		DiscrepancyNote: discNote,
		RawOutput:       rawAssessmentJSON,
		PromptVersion:   params.PromptVersion,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("inserting commit assessment: %w", err)
	}

	// 4. Link session to entry and mark status = 'committed'
	_, err = qtx.LinkSessionToEntry(ctx, sqlc.LinkSessionToEntryParams{
		ID:      params.SessionID,
		EntryID: uuid.NullUUID{UUID: entryRow.ID, Valid: true},
		UserID:  params.UserID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("linking session to entry: %w", err)
	}

	// 5. Commit atomic transaction
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("committing atomic transaction: %w", err)
	}

	mappedEntry := mapVocabEntry(entryRow)
	var asstDisc *string
	if assessmentRow.DiscrepancyNote.Valid {
		d := assessmentRow.DiscrepancyNote.String
		asstDisc = &d
	}

	mappedAssessment := &CommitAssessment{
		SessionID:       assessmentRow.SessionID,
		Verdict:         assessmentRow.Verdict,
		DiscrepancyNote: asstDisc,
		RawOutput:       params.AssessmentRaw,
		PromptVersion:   assessmentRow.PromptVersion,
		CreatedAt:       assessmentRow.CreatedAt,
	}

	return mappedEntry, mappedAssessment, nil
}

func (r *sqlRepository) GetEntryByID(ctx context.Context, id, userID uuid.UUID) (*VocabEntry, error) {
	entry, err := r.q.GetVocabEntryByID(ctx, sqlc.GetVocabEntryByIDParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEntryNotFound
		}
		return nil, fmt.Errorf("getting vocab entry: %w", err)
	}
	return mapVocabEntry(entry), nil
}

func (r *sqlRepository) GetEntryRevisions(ctx context.Context, entryID uuid.UUID) ([]EntryRevision, error) {
	dbRevs, err := r.q.ListEntryRevisions(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("listing revisions for entry: %w", err)
	}

	revisions := make([]EntryRevision, 0, len(dbRevs))
	for _, rev := range dbRevs {
		mapped, err := mapEntryRevision(rev)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, *mapped)
	}
	return revisions, nil
}

func (r *sqlRepository) ListEntries(ctx context.Context, userID uuid.UUID, filter ListEntriesFilter) ([]VocabEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}

	var startNull, endNull sql.NullTime
	if filter.StartDate != nil {
		startNull = sql.NullTime{Time: *filter.StartDate, Valid: true}
	}
	if filter.EndDate != nil {
		endNull = sql.NullTime{Time: *filter.EndDate, Valid: true}
	}

	rows, err := r.q.FilterVocabEntries(ctx, sqlc.FilterVocabEntriesParams{
		UserID:            userID,
		Limit:             limit,
		Offset:            filter.Offset,
		PartOfSpeech:      sql.NullString{String: filter.PartOfSpeech, Valid: filter.PartOfSpeech != ""},
		UnderstandingFlag: sql.NullString{String: filter.UnderstandingFlag, Valid: filter.UnderstandingFlag != ""},
		StartDate:         startNull,
		EndDate:           endNull,
	})
	if err != nil {
		return nil, fmt.Errorf("filtering vocab entries: %w", err)
	}

	entries := make([]VocabEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, *mapVocabEntry(row))
	}
	return entries, nil
}

func (r *sqlRepository) SearchEntries(ctx context.Context, userID uuid.UUID, query string, limit, offset int32) ([]VocabEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := r.q.SearchVocabEntries(ctx, sqlc.SearchVocabEntriesParams{
		UserID:             userID,
		WebsearchToTsquery: query,
		Limit:              limit,
		Offset:             offset,
	})
	if err != nil {
		return nil, fmt.Errorf("full-text search query execution: %w", err)
	}

	entries := make([]VocabEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, *mapVocabEntry(row))
	}
	return entries, nil
}

func (r *sqlRepository) UpdateEntryPersonalTx(ctx context.Context, userID, entryID uuid.UUID, req UpdateEntryRequest) (*VocabEntry, *EntryRevision, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("beginning update transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := r.q.WithTx(tx)

	// Fetch existing record to verify tenancy and compute revision changes
	existing, err := qtx.GetVocabEntryByID(ctx, sqlc.GetVocabEntryByIDParams{
		ID:     entryID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrEntryNotFound
		}
		return nil, nil, fmt.Errorf("fetching entry for update: %w", err)
	}

	// Apply updates
	personalDef := existing.PersonalDefinition
	if req.PersonalDefinition != nil {
		personalDef = *req.PersonalDefinition
	}

	example := existing.ExampleSentence.String
	if req.ExampleSentence != nil {
		example = *req.ExampleSentence
	}

	notes := existing.ContextualNotes.String
	if req.ContextualNotes != nil {
		notes = *req.ContextualNotes
	}

	flag := existing.UnderstandingFlag
	if req.UnderstandingFlag != nil {
		flag = string(*req.UnderstandingFlag)
	}

	discrepancy := existing.DiscrepancyNote.String
	if req.DiscrepancyNote != nil {
		discrepancy = *req.DiscrepancyNote
	}

	// Execute entry update
	updated, err := qtx.UpdateVocabEntryPersonal(ctx, sqlc.UpdateVocabEntryPersonalParams{
		ID:                 entryID,
		UserID:             userID,
		PersonalDefinition: personalDef,
		ExampleSentence:    sql.NullString{String: example, Valid: example != ""},
		ContextualNotes:    sql.NullString{String: notes, Valid: notes != ""},
		UnderstandingFlag:  flag,
		DiscrepancyNote:    sql.NullString{String: discrepancy, Valid: discrepancy != ""},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("updating vocab entry: %w", err)
	}

	// Compute revision changes diff
	changes := map[string]interface{}{
		"personal_definition": personalDef,
		"example_sentence":    example,
		"contextual_notes":    notes,
		"understanding_flag":  flag,
		"revision_type":       "user_edit",
	}
	changesJSON, err := json.Marshal(changes)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling revision changes: %w", err)
	}

	nextRev, err := qtx.GetNextRevisionNo(ctx, entryID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting next revision number: %w", err)
	}

	dbRev, err := qtx.CreateEntryRevision(ctx, sqlc.CreateEntryRevisionParams{
		EntryID: entryID,
		RevNo:   nextRev,
		Changes: changesJSON,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("creating entry revision: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("committing update transaction: %w", err)
	}

	mappedEntry := mapVocabEntry(updated)
	mappedRev, err := mapEntryRevision(dbRev)
	if err != nil {
		return nil, nil, err
	}

	return mappedEntry, mappedRev, nil
}

func (r *sqlRepository) DeleteEntry(ctx context.Context, id, userID uuid.UUID) error {
	err := r.q.DeleteVocabEntry(ctx, sqlc.DeleteVocabEntryParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("deleting vocab entry: %w", err)
	}
	return nil
}
