package guests

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jadeejoao/jadeejoao-api/internal/guests/guestsdb"
)

// pgRepo adapts the sqlc-generated queries to the Repo interface.
type pgRepo struct {
	pool *pgxpool.Pool
	q    *guestsdb.Queries
}

// NewRepo builds the Postgres-backed guests repository.
func NewRepo(pool *pgxpool.Pool) Repo {
	return &pgRepo{pool: pool, q: guestsdb.New(pool)}
}

func (r *pgRepo) FindGuestByNormalizedName(ctx context.Context, normalized string) (Member, uuid.UUID, error) {
	row, err := r.q.GetGuestByNormalizedName(ctx, normalized)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, uuid.Nil, ErrNotFound
	}
	if err != nil {
		return Member{}, uuid.Nil, err
	}
	member := Member{ID: row.ID, FullName: row.FullName, IsPrimary: row.IsPrimary, Category: row.Category, Attending: row.Attending}
	return member, row.GroupID, nil
}

func (r *pgRepo) GetGroup(ctx context.Context, id uuid.UUID) (Group, error) {
	row, err := r.q.GetGuestGroup(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	if err != nil {
		return Group{}, err
	}
	return Group{ID: row.ID, Label: row.Label}, nil
}

func (r *pgRepo) ListMembers(ctx context.Context, groupID uuid.UUID) ([]Member, error) {
	rows, err := r.q.ListGroupMembers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, len(rows))
	for i, row := range rows {
		out[i] = Member{
			ID: row.ID, FullName: row.FullName, IsPrimary: row.IsPrimary,
			Category: row.Category, Attending: row.Attending, AddedByGuest: row.AddedByGuest,
			Gender: row.Gender, Side: row.Side, Circle: row.Circle,
			CeremonyRole: row.CeremonyRole, Notes: row.Notes,
		}
	}
	return out, nil
}

func (r *pgRepo) SuggestNames(ctx context.Context, normalizedPrefix string) ([]string, error) {
	return r.q.SuggestGuestNames(ctx, normalizedPrefix)
}

func (r *pgRepo) ListAllGroups(ctx context.Context) ([]Group, error) {
	rows, err := r.q.ListAllGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Group, len(rows))
	for i, row := range rows {
		out[i] = Group{ID: row.ID, Label: row.Label}
	}
	return out, nil
}

func (r *pgRepo) ListAllGuests(ctx context.Context) (map[uuid.UUID][]Member, error) {
	rows, err := r.q.ListAllGuests(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]Member)
	for _, row := range rows {
		out[row.GroupID] = append(out[row.GroupID], Member{
			ID: row.ID, FullName: row.FullName, IsPrimary: row.IsPrimary,
			Category: row.Category, Attending: row.Attending, AddedByGuest: row.AddedByGuest,
			Gender: row.Gender, Side: row.Side, Circle: row.Circle,
			CeremonyRole: row.CeremonyRole, Notes: row.Notes,
		})
	}
	return out, nil
}

// UpdateAttendances applies every answer in one transaction; any update that
// does not match exactly one guest of the group aborts the whole submission.
// SuggestAvailableCompanions lists guests this invitation may gather in.
func (r *pgRepo) SuggestAvailableCompanions(ctx context.Context, groupID uuid.UUID, prefix string) ([]CompanionOption, error) {
	rows, err := r.q.SuggestAvailableCompanions(ctx, guestsdb.SuggestAvailableCompanionsParams{
		Prefix: prefix, GroupID: groupID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]CompanionOption, len(rows))
	for i, row := range rows {
		out[i] = CompanionOption{ID: row.ID, FullName: row.FullName}
	}
	return out, nil
}

// AddCompanion moves someone the couple already invited into this invitation,
// refusing once it has spent its allowance.
//
// Everything shares one transaction holding a row lock on the destination
// group. The cheap version — check, then move — lets two phones open the same
// invitation and each be told there is one slot left; worse, it lets two
// invitations claim the same person at the same moment.
func (r *pgRepo) AddCompanion(ctx context.Context, groupID, guestID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	q := r.q.WithTx(tx)
	if _, err := q.LockGroupForCompanion(ctx, groupID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	already, err := q.CountGroupCompanions(ctx, groupID)
	if err != nil {
		return err
	}
	if already >= MaxCompanionsPerGroup {
		return ErrCompanionLimit
	}

	guest, err := q.GetGuestWithGroupSize(ctx, guestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if guest.GroupID == groupID {
		return ErrAlreadyOnInvitation
	}
	// Heading an invitation that holds other people means moving would orphan
	// the rest of that family. Read inside the transaction so a concurrent
	// move cannot make this stale between the check and the write.
	if guest.GroupSize > 1 {
		return ErrGuestUnavailable
	}

	from := guest.GroupID
	moved, err := q.MoveGuestToGroup(ctx, guestsdb.MoveGuestToGroupParams{ID: guestID, GroupID: groupID})
	if err != nil {
		return err
	}
	if moved != 1 {
		return ErrNotFound
	}
	// Their old invitation now holds nobody. Leaving it behind would litter
	// the couple's dashboard with empty rows.
	if err := q.DeleteGroupIfEmpty(ctx, from); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RemoveCompanion sends someone back to the invitation they arrived with,
// rather than deleting them: they are still invited, just no longer part of
// this group. A miss means the id belongs to someone the couple placed here
// (or to another invitation), which is a refusal, not a not-found.
func (r *pgRepo) RemoveCompanion(ctx context.Context, groupID, guestID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	q := r.q.WithTx(tx)
	guest, err := q.GetGuestWithGroupSize(ctx, guestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotRemovable
		}
		return err
	}

	// The invitation is recreated under their own name — the same label the
	// import gave it — so leaving and being re-added round-trips exactly.
	home, err := q.InsertGuestGroup(ctx, guest.FullName)
	if err != nil {
		return err
	}
	restored, err := q.RestoreCompanionToOwnGroup(ctx, guestsdb.RestoreCompanionToOwnGroupParams{
		ID: guestID, GroupID: groupID, NewGroupID: home,
	})
	if err != nil {
		return err
	}
	if restored == 0 {
		// Rolls back the group created just above.
		return ErrNotRemovable
	}
	return tx.Commit(ctx)
}

// UpdateGuestDetails rewrites one person's identity fields from the panel.
// normalized_name is recomputed here, never taken from the caller: it is what
// the guest lookup matches on, and a stale one would make somebody vanish from
// their own invitation.
func (r *pgRepo) UpdateGuestDetails(ctx context.Context, guestID uuid.UUID, edit GuestEdit) error {
	affected, err := r.q.UpdateGuestDetails(ctx, guestsdb.UpdateGuestDetailsParams{
		ID:             guestID,
		FullName:       edit.FullName,
		NormalizedName: Normalize(edit.FullName),
		Category:       edit.Category,
		Gender:         edit.Gender,
		Side:           edit.Side,
		Circle:         edit.Circle,
		CeremonyRole:   edit.CeremonyRole,
		Notes:          edit.Notes,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrNameTaken
		}
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteGuest removes one person, and their invitation with them if it is left
// empty — an invitation with nobody on it is only clutter in the dashboard.
func (r *pgRepo) DeleteGuest(ctx context.Context, guestID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	q := r.q.WithTx(tx)
	guest, err := q.GetGuestWithGroupSize(ctx, guestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	affected, err := q.DeleteGuest(ctx, guestID)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	if err := q.DeleteGroupIfEmpty(ctx, guest.GroupID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *pgRepo) RenameGroup(ctx context.Context, groupID uuid.UUID, label string) error {
	affected, err := r.q.RenameGroup(ctx, guestsdb.RenameGroupParams{ID: groupID, Label: label})
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetGroupPrimary hands the invitation to somebody else. One statement, so no
// instant exists where the invitation has two primaries or none.
func (r *pgRepo) SetGroupPrimary(ctx context.Context, groupID, guestID uuid.UUID) error {
	affected, err := r.q.SetGroupPrimary(ctx, guestsdb.SetGroupPrimaryParams{
		GuestID: guestID, GroupID: groupID,
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// MoveGuestAsAdmin merges someone into another invitation. Unlike the guest
// path this accepts anyone — the couple is allowed to reshape their own list —
// but it still tidies the invitation left behind.
func (r *pgRepo) MoveGuestAsAdmin(ctx context.Context, groupID, guestID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	q := r.q.WithTx(tx)
	guest, err := q.GetGuestWithGroupSize(ctx, guestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if guest.GroupID == groupID {
		return ErrAlreadyOnInvitation
	}
	if _, err := q.GetGuestGroup(ctx, groupID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	moved, err := q.MoveGuestToGroupAsAdmin(ctx, guestsdb.MoveGuestToGroupAsAdminParams{
		ID: guestID, GroupID: groupID,
	})
	if err != nil {
		return err
	}
	if moved == 0 {
		return ErrNotFound
	}
	if err := q.DeleteGroupIfEmpty(ctx, guest.GroupID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *pgRepo) UpdateAttendances(ctx context.Context, groupID uuid.UUID, updates []AttendanceUpdate) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	q := r.q.WithTx(tx)
	for _, u := range updates {
		affected, err := q.UpdateGuestAttendance(ctx, guestsdb.UpdateGuestAttendanceParams{
			ID:        u.GuestID,
			GroupID:   groupID,
			Attending: u.Attending,
		})
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: guest %s", ErrInvalidMembers, u.GuestID)
		}
	}
	return tx.Commit(ctx)
}
