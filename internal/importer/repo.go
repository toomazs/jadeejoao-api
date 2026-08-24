package importer

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jadeejoao/jadeejoao-api/internal/importer/importerdb"
)

// pgRepo adapts the sqlc-generated queries to the Repo interface.
type pgRepo struct {
	pool *pgxpool.Pool
	q    *importerdb.Queries
}

// NewRepo builds the Postgres-backed importer repository.
func NewRepo(pool *pgxpool.Pool) Repo {
	return &pgRepo{pool: pool, q: importerdb.New(pool)}
}

func (r *pgRepo) Snapshot(ctx context.Context) (Snapshot, error) {
	groups, err := r.q.ListGroupsForImport(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	guests, err := r.q.ListGuestsForImport(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snap := Snapshot{
		Groups: make([]ExistingGroup, len(groups)),
		Guests: make([]ExistingGuest, len(guests)),
	}
	for i, g := range groups {
		snap.Groups[i] = ExistingGroup{ID: g.ID, Label: g.Label}
	}
	for i, g := range guests {
		snap.Guests[i] = ExistingGuest{
			ID: g.ID, GroupID: g.GroupID, FullName: g.FullName,
			NormalizedName: g.NormalizedName, IsPrimary: g.IsPrimary, Category: g.Category,
		}
	}
	return snap, nil
}

// Apply executes the whole plan in one transaction. It writes identity fields
// only — attendance belongs to the guests service and is never touched here
// (AD-10).
func (r *pgRepo) Apply(ctx context.Context, plan Plan, replace bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	q := r.q.WithTx(tx)
	// Inside the same transaction as the writes: a wipe that commits without
	// its replacement would leave the couple with no guest list at all.
	if replace {
		if err := q.DeleteAllGuestGroups(ctx); err != nil {
			return fmt.Errorf("clear guest list: %w", err)
		}
	}
	for _, group := range plan.Groups {
		var groupID uuid.UUID
		if group.ExistingID != nil {
			groupID = *group.ExistingID
			if group.UpdateLabel {
				if err := q.UpdateGroupLabel(ctx, importerdb.UpdateGroupLabelParams{ID: groupID, Label: group.Label}); err != nil {
					return fmt.Errorf("rename group to %q: %w", group.Label, err)
				}
			}
		} else {
			created, err := q.InsertGuestGroup(ctx, group.Label)
			if err != nil {
				return fmt.Errorf("create group %q: %w", group.Label, err)
			}
			groupID = created
		}

		var primaryID uuid.UUID
		for _, guest := range group.Guests {
			var guestID uuid.UUID
			if guest.ExistingID != nil {
				guestID = *guest.ExistingID
				if err := q.UpdateGuestIdentity(ctx, importerdb.UpdateGuestIdentityParams{
					ID: guestID, FullName: guest.FullName, Category: guest.Category,
					Gender: guest.Gender, Side: guest.Side, Circle: guest.Circle,
					CeremonyRole: guest.CeremonyRole, Notes: guest.Notes,
				}); err != nil {
					return fmt.Errorf("update guest %q: %w", guest.FullName, err)
				}
			} else {
				inserted, err := q.InsertImportedGuest(ctx, importerdb.InsertImportedGuestParams{
					GroupID: groupID, FullName: guest.FullName, NormalizedName: guest.NormalizedName,
					IsPrimary: false, Category: guest.Category,
					Gender: guest.Gender, Side: guest.Side, Circle: guest.Circle,
					CeremonyRole: guest.CeremonyRole, Notes: guest.Notes,
				})
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == "23505" {
					// normalized_name UNIQUE tripped: the list changed
					// between snapshot and apply. Roll everything back and
					// let the admin retry against fresh state.
					return fmt.Errorf("%w: %s", ErrNameCollision, guest.FullName)
				}
				if err != nil {
					return fmt.Errorf("insert guest %q: %w", guest.FullName, err)
				}
				guestID = inserted
			}
			if guest.IsPrimary {
				primaryID = guestID
			}
		}

		// One exclusive statement keeps a single primary per group, covering
		// DB members the file did not mention. Only when the plan says so —
		// partial uploads must not silently reassign an existing primary.
		if group.EnforcePrimary && primaryID != uuid.Nil {
			if err := q.SetGroupPrimary(ctx, importerdb.SetGroupPrimaryParams{
				GuestID: primaryID, GroupID: groupID,
			}); err != nil {
				return fmt.Errorf("set primary for group %q: %w", group.Label, err)
			}
		}
	}
	return tx.Commit(ctx)
}

func (r *pgRepo) SaveReport(ctx context.Context, report []byte) error {
	_, err := r.q.InsertImportReport(ctx, report)
	return err
}
