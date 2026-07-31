package auth

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
)

// PostgresRepository implémente Repository sur les requêtes générées.
type PostgresRepository struct {
	q *sqlc.Queries
}

var _ Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(q *sqlc.Queries) *PostgresRepository {
	return &PostgresRepository{q: q}
}

// ─── Utilisateurs ────────────────────────────────────────────────────────────

func (r *PostgresRepository) CountUsers(ctx context.Context) (int64, error) {
	return r.q.CountUsers(ctx)
}

func (r *PostgresRepository) CreateUser(ctx context.Context, u User, passwordHash string) (User, error) {
	var email *string
	if u.Email != "" {
		email = &u.Email
	}
	var displayName *string
	if u.DisplayName != "" {
		displayName = &u.DisplayName
	}

	row, err := r.q.CreateUser(ctx, sqlc.CreateUserParams{
		ID:           u.ID,
		Username:     u.Username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         sqlc.UserRole(u.Role),
		DisplayName:  displayName,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUserExists
		}
		return User{}, err
	}
	return userFromRow(row), nil
}

func (r *PostgresRepository) GetUser(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	return userFromRow(row), nil
}

// GetUserByUsername retourne l'utilisateur et son hachage de mot de passe.
//
// Le hachage n'est retourné que par cette méthode, appelée uniquement au
// moment de la connexion : il ne circule nulle part ailleurs.
func (r *PostgresRepository) GetUserByUsername(ctx context.Context, username string) (User, string, error) {
	row, err := r.q.GetUserByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", err
	}
	return userFromRow(row), row.PasswordHash, nil
}

func (r *PostgresRepository) SetUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	return r.q.SetUserPassword(ctx, sqlc.SetUserPasswordParams{ID: id, PasswordHash: passwordHash})
}

func (r *PostgresRepository) TouchUserLogin(ctx context.Context, id uuid.UUID) error {
	return r.q.TouchUserLogin(ctx, id)
}

func userFromRow(row sqlc.User) User {
	u := User{
		ID:           row.ID,
		Username:     row.Username,
		Role:         string(row.Role),
		Restricted:   row.Restricted,
		MaxAgeRating: row.MaxAgeRating,
	}
	if row.Email != nil {
		u.Email = *row.Email
	}
	if row.DisplayName != nil {
		u.DisplayName = *row.DisplayName
	}
	return u
}

// ─── Appareils ───────────────────────────────────────────────────────────────

func (r *PostgresRepository) UpsertDevice(ctx context.Context, d Device) (Device, error) {
	var appVersion *string
	if d.AppVersion != "" {
		appVersion = &d.AppVersion
	}

	row, err := r.q.UpsertDevice(ctx, sqlc.UpsertDeviceParams{
		ID:         d.ID,
		UserID:     d.UserID,
		Name:       d.Name,
		Platform:   sqlc.DevicePlatform(d.Platform),
		AppVersion: appVersion,
	})
	if err != nil {
		return Device{}, err
	}
	return deviceFromRow(row), nil
}

func (r *PostgresRepository) ListDevices(ctx context.Context, userID uuid.UUID) ([]Device, error) {
	rows, err := r.q.ListDevicesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make([]Device, 0, len(rows))
	for _, row := range rows {
		out = append(out, deviceFromRow(row))
	}
	return out, nil
}

func (r *PostgresRepository) DeleteDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	return r.q.DeleteDevice(ctx, sqlc.DeleteDeviceParams{ID: deviceID, UserID: userID})
}

func (r *PostgresRepository) DeviceExists(ctx context.Context, userID, deviceID uuid.UUID) (bool, error) {
	return r.q.DeviceExists(ctx, sqlc.DeviceExistsParams{ID: deviceID, UserID: userID})
}

func (r *PostgresRepository) RevokeDeviceSessions(ctx context.Context, userID, deviceID uuid.UUID) (int64, error) {
	return r.q.RevokeDeviceSessions(ctx, sqlc.RevokeDeviceSessionsParams{
		UserID:   userID,
		DeviceID: uuid.NullUUID{UUID: deviceID, Valid: true},
	})
}

func deviceFromRow(row sqlc.Device) Device {
	d := Device{
		ID:       row.ID,
		UserID:   row.UserID,
		Name:     row.Name,
		Platform: string(row.Platform),
	}
	if row.AppVersion != nil {
		d.AppVersion = *row.AppVersion
	}
	if row.LastSeenAt.Valid {
		d.LastSeenAt = row.LastSeenAt.Time
	}
	return d
}

// ─── Sessions ────────────────────────────────────────────────────────────────

func (r *PostgresRepository) CreateSession(ctx context.Context, s Session, tokenHash []byte, userAgent string, ip *netip.Addr) (Session, error) {
	var ua *string
	if userAgent != "" {
		ua = &userAgent
	}

	var deviceID uuid.NullUUID
	if s.DeviceID != nil {
		deviceID = uuid.NullUUID{UUID: *s.DeviceID, Valid: true}
	}
	var parentID uuid.NullUUID
	if s.ParentID != nil {
		parentID = uuid.NullUUID{UUID: *s.ParentID, Valid: true}
	}

	row, err := r.q.CreateSession(ctx, sqlc.CreateSessionParams{
		ID:        s.ID,
		UserID:    s.UserID,
		DeviceID:  deviceID,
		TokenHash: tokenHash,
		ParentID:  parentID,
		ExpiresAt: pgtype.Timestamptz{Time: s.ExpiresAt, Valid: true},
		UserAgent: ua,
		IP:        ipFrom(ip),
	})
	if err != nil {
		return Session{}, fmt.Errorf("auth : création de session : %w", err)
	}
	return sessionFromRow(row), nil
}

func (r *PostgresRepository) GetSessionByTokenHash(ctx context.Context, hash []byte) (Session, error) {
	row, err := r.q.GetSessionByTokenHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrTokenInvalid
	}
	if err != nil {
		return Session{}, err
	}
	return sessionFromRow(row), nil
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, id uuid.UUID) error {
	return r.q.RevokeSession(ctx, id)
}

func (r *PostgresRepository) RevokeSessionChain(ctx context.Context, id uuid.UUID) (int64, error) {
	return r.q.RevokeSessionChain(ctx, id)
}

func (r *PostgresRepository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.q.RevokeAllUserSessions(ctx, userID)
}

func sessionFromRow(row sqlc.Session) Session {
	s := Session{
		ID:     row.ID,
		UserID: row.UserID,
	}
	if row.DeviceID.Valid {
		id := row.DeviceID.UUID
		s.DeviceID = &id
	}
	if row.ParentID.Valid {
		id := row.ParentID.UUID
		s.ParentID = &id
	}
	if row.ExpiresAt.Valid {
		s.ExpiresAt = row.ExpiresAt.Time
	}
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		s.RevokedAt = &t
	}
	return s
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func ipFrom(addr *netip.Addr) *netip.Addr {
	if addr == nil || !addr.IsValid() {
		return nil
	}
	return addr
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
