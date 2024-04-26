package store

import (
	"context"

	"github.com/google/uuid"
)

const sqlSelectUserByExternalID = `
SELECT 
    id,
    first_name,
    last_name,
    external_id
FROM users
WHERE external_id = $1`

func (s *Store) GetUserByExternalID(ctx context.Context, externalID uuid.UUID) (User, error) {
	var user User
	err := s.db.GetContext(ctx, &user, sqlSelectUserByExternalID, externalID)
	if err != nil {
		return User{}, err
	}
	return user, nil
}
