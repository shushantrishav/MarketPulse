package redis

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
	"marketpulse/internal/config"
)

type Client struct {
	rdb *redis.Client
	cfg *config.Config
	down bool
}

func NewClient(cfg *config.Config) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       cfg.RedisDB,
	})
	return &Client{rdb: rdb, cfg: cfg}
}

func (c *Client) Ping(ctx context.Context) error {
	if err := c.rdb.Ping(ctx).Err(); err != nil {
		c.down = true
		return err
	}
	if c.down {
		c.down = false
		log.Println("Redis recovered")
	}
	return nil
}

func (c *Client) RDB() *redis.Client { return c.rdb }
func (c *Client) IsDown() bool       { return c.down }
