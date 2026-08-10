package messages

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jadeejoao/jadeejoao-api/internal/messages/messagesdb"
)

// pgRepo adapts the sqlc-generated queries to the Repo interface.
type pgRepo struct {
	q *messagesdb.Queries
}

// NewRepo builds the Postgres-backed messages repository.
func NewRepo(pool *pgxpool.Pool) Repo {
	return &pgRepo{q: messagesdb.New(pool)}
}

func (r *pgRepo) Insert(ctx context.Context, groupID *uuid.UUID, authorName, body string) (Message, error) {
	gid := uuid.NullUUID{}
	if groupID != nil {
		gid = uuid.NullUUID{UUID: *groupID, Valid: true}
	}
	row, err := r.q.InsertMessage(ctx, messagesdb.InsertMessageParams{
		GroupID:    gid,
		AuthorName: authorName,
		Body:       body,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return Message{}, ErrUnknownGroup
	}
	if err != nil {
		return Message{}, err
	}
	return fromRow(row.ID, row.GroupID, row.AuthorName, row.Body, row.Status, row.CreatedAt), nil
}

func (r *pgRepo) List(ctx context.Context, statusFilter string) ([]Message, error) {
	rows, err := r.q.ListMessages(ctx, statusFilter)
	if err != nil {
		return nil, err
	}
	out := make([]Message, len(rows))
	for i, row := range rows {
		out[i] = fromRow(row.ID, row.GroupID, row.AuthorName, row.Body, row.Status, row.CreatedAt)
	}
	return out, nil
}

func (r *pgRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (Message, error) {
	row, err := r.q.UpdateMessageStatus(ctx, messagesdb.UpdateMessageStatusParams{ID: id, Status: status})
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	if err != nil {
		return Message{}, err
	}
	return fromRow(row.ID, row.GroupID, row.AuthorName, row.Body, row.Status, row.CreatedAt), nil
}

func fromRow(id uuid.UUID, groupID uuid.NullUUID, author, body, status string, createdAt time.Time) Message {
	m := Message{ID: id, AuthorName: author, Body: body, Status: status, CreatedAt: createdAt}
	if groupID.Valid {
		gid := groupID.UUID
		m.GroupID = &gid
	}
	return m
}
