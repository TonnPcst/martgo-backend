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

// Key must match the auth service (martgo-auth: internal/session.Key).
func Key(userID int64) string { return "session:user:" + strconv.FormatInt(userID, 10) }

type Reader struct {
	rdb *redis.Client
}

func NewReader(rdb *redis.Client) *Reader { return &Reader{rdb: rdb} }

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
