package seed

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/warmbly/warmbly/internal/app/user"
)

// ForgetCachedUsers drops every cached /auth/me copy so rows the seed just
// rewrote (onboarding, names, admin bits) are read fresh on the next request.
// Best effort: the seed only needs Postgres, so no REDIS means nothing to do.
func ForgetCachedUsers(ctx context.Context, pool *pgxpool.Pool) {
	endpoint := strings.TrimSpace(os.Getenv("REDIS"))
	if endpoint == "" {
		return
	}
	opts, err := redis.ParseURL(endpoint)
	if err != nil {
		log.Printf("seed: REDIS is not a valid URL, cached users were left alone: %v", err)
		return
	}
	rdb := redis.NewClient(opts)
	defer rdb.Close()

	rows, err := pool.Query(ctx, `SELECT id FROM users`)
	if err != nil {
		log.Printf("seed: listing users to uncache: %v", err)
		return
	}
	var keys []string
	for rows.Next() {
		var id uuid.UUID
		if rows.Scan(&id) == nil {
			keys = append(keys, user.CacheKey(id))
		}
	}
	rows.Close()
	if len(keys) == 0 {
		return
	}
	if err := rdb.Del(ctx, keys...).Err(); err != nil {
		log.Printf("seed: could not clear cached users, a signed-in session may see stale data for up to %s: %v", user.UserTTL, err)
	}
}
