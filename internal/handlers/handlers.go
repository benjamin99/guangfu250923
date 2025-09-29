package handlers

import "github.com/jackc/pgx/v5/pgxpool"

type Handler struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }
