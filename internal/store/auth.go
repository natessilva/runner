package store

import (
	"database/sql"
	"errors"
	"time"
)

// ErrNoAuth is returned when no authentication is stored
var ErrNoAuth = errors.New("no authentication stored")

// GetAuth retrieves the stored authentication tokens
func (db *DB) GetAuth() (*Auth, error) {
	row := db.QueryRow(`
		SELECT athlete_id, access_token, refresh_token, expires_at
		FROM auth
		WHERE id = 1
	`)

	var auth Auth
	var expiresAt int64
	err := row.Scan(&auth.AthleteID, &auth.AccessToken, &auth.RefreshToken, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoAuth
	}
	if err != nil {
		return nil, err
	}

	auth.ExpiresAt = time.Unix(expiresAt, 0)
	return &auth, nil
}

// SaveAuth stores or updates the authentication tokens
func (db *DB) SaveAuth(auth *Auth) error {
	_, err := db.Exec(`
		INSERT INTO auth (id, athlete_id, access_token, refresh_token, expires_at, updated_at)
		VALUES (1, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			athlete_id = excluded.athlete_id,
			access_token = excluded.access_token,
			refresh_token = excluded.refresh_token,
			expires_at = excluded.expires_at,
			updated_at = CURRENT_TIMESTAMP
	`, auth.AthleteID, auth.AccessToken, auth.RefreshToken, auth.ExpiresAt.Unix())
	return err
}

// UpdateTokens updates just the access and refresh tokens
func (db *DB) UpdateTokens(accessToken, refreshToken string, expiresAt time.Time) error {
	result, err := db.Exec(`
		UPDATE auth
		SET access_token = ?, refresh_token = ?, expires_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, accessToken, refreshToken, expiresAt.Unix())
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNoAuth
	}
	return nil
}
