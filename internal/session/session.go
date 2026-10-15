// Package session reads the per-user token state that the auth service
// publishes to Redis, so revocation is checked without calling that service.
package session

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("session not found")

type State struct {
	TokenVersion int32
	Role         string
	Active       bool
}

// Key returns the Redis key holding a user's session state.
//
// ENG: Must match the auth service (martgo-auth: internal/session.Key), which writes
// the entry that this service reads.
// LAO: ສ້າງ key ຂອງ Redis ສຳລັບ session ຂອງຜູ້ໃຊ້ — ຕ້ອງກົງກັບ auth service.
//
// @param userID - user ID
// @returns the key, e.g. "session:user:42"
func Key(userID int64) string { return "session:user:" + strconv.FormatInt(userID, 10) }

type Reader struct {
	rdb *redis.Client
}

// NewReader creates a session reader on top of a Redis client.
//
// ENG: The reader is read-only; only the auth service writes sessions.
// LAO: ສ້າງຕົວອ່ານ session ຈາກ Redis (ອ່ານຢ່າງດຽວ).
//
// @param rdb - Redis client
// @returns the reader
func NewReader(rdb *redis.Client) *Reader { return &Reader{rdb: rdb} }

// Get loads a user's current token state from Redis.
//
// ENG: Reads the "ver", "role" and "active" fields of the session hash. A missing
// entry means the user has not logged in since the last token expired.
// LAO: ອ່ານສະຖານະ session ຂອງຜູ້ໃຊ້ (ເວີຊັນ token, role, active) ຈາກ Redis.
//
// @param ctx - request context
// @param userID - user ID from the token
// @returns the state, ErrNotFound if there is no entry, or an error if Redis fails or a field is corrupt
func (r *Reader) Get(ctx context.Context, userID int64) (State, error) {
	vals, err := r.rdb.HGetAll(ctx, Key(userID)).Result()
	if err != nil {
		return State{}, err
	}
	if len(vals) == 0 {
		return State{}, ErrNotFound
	}
	ver, err := strconv.ParseInt(vals["ver"], 10, 32)
	if err != nil {
		return State{}, fmt.Errorf("session ver: %w", err)
	}
	active, err := strconv.ParseBool(vals["active"])
	if err != nil {
		return State{}, fmt.Errorf("session active: %w", err)
	}
	return State{TokenVersion: int32(ver), Role: vals["role"], Active: active}, nil
}
