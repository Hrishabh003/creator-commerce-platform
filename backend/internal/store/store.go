// Package store holds all PostgreSQL access. List queries hydrate related rows
// with one batched query each (WHERE id = ANY($1)) instead of one query per row.
package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Hrishabh003/creator-commerce-platform/backend/internal/ai"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrNotPending = errors.New("draft is not pending review")
)

type Store struct{ Pool *pgxpool.Pool }

type Product struct {
	ID         int64  `json:"id"`
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	PriceCents int    `json:"priceCents"`
	Stock      int    `json:"stock"`
}

type Customer struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Order struct {
	ID         int64     `json:"id"`
	CustomerID int64     `json:"customerId"`
	Status     string    `json:"status"`
	TotalCents int       `json:"totalCents"`
	CreatedAt  time.Time `json:"createdAt"`
	Customer   *Customer `json:"customer"`
}

type Draft struct {
	ID        int64     `json:"id"`
	TicketID  int64     `json:"ticketId"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	Sources   []string  `json:"sources"`
	CreatedAt time.Time `json:"createdAt"`
}

type Ticket struct {
	ID         int64     `json:"id"`
	CustomerID int64     `json:"customerId"`
	Subject    string    `json:"subject"`
	Body       string    `json:"body"`
	Category   string    `json:"category"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	Customer   *Customer `json:"customer"`
	Drafts     []Draft   `json:"drafts"`
}

type Campaign struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Subject      string     `json:"subject"`
	Status       string     `json:"status"`
	SentCount    int        `json:"sentCount"`
	ScheduledFor *time.Time `json:"scheduledFor"`
}

func (s *Store) Products(ctx context.Context) ([]Product, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, sku, name, price_cents, stock FROM products ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.PriceCents, &p.Stock); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStock(ctx context.Context, id int64, stock int) (*Product, error) {
	var p Product
	err := s.Pool.QueryRow(ctx,
		`UPDATE products SET stock = $2 WHERE id = $1 RETURNING id, sku, name, price_cents, stock`,
		id, stock).Scan(&p.ID, &p.SKU, &p.Name, &p.PriceCents, &p.Stock)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

// CustomersByID loads many customers in a single query.
func (s *Store) CustomersByID(ctx context.Context, ids []int64) (map[int64]*Customer, error) {
	out := map[int64]*Customer{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT id, name, email FROM customers WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c := &Customer{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Email); err != nil {
			return nil, err
		}
		out[c.ID] = c
	}
	return out, rows.Err()
}

func (s *Store) Orders(ctx context.Context, status string, limit int) ([]Order, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, customer_id, status, total_cents, created_at FROM orders
		 WHERE ($1::text = '' OR status = $1::text) ORDER BY created_at DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	ids := []int64{}
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.Status, &o.TotalCents, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
		ids = append(ids, o.CustomerID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	customers, err := s.CustomersByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Customer = customers[out[i].CustomerID]
	}
	return out, nil
}

// DraftsByTicket loads drafts for many tickets in a single query.
func (s *Store) DraftsByTicket(ctx context.Context, ticketIDs []int64) (map[int64][]Draft, error) {
	out := map[int64][]Draft{}
	if len(ticketIDs) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT id, ticket_id, body, status, sources, created_at FROM ticket_drafts
		 WHERE ticket_id = ANY($1) ORDER BY created_at DESC`, ticketIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d Draft
		if err := rows.Scan(&d.ID, &d.TicketID, &d.Body, &d.Status, &d.Sources, &d.CreatedAt); err != nil {
			return nil, err
		}
		out[d.TicketID] = append(out[d.TicketID], d)
	}
	return out, rows.Err()
}

func (s *Store) hydrateTickets(ctx context.Context, ts []Ticket) error {
	cids := make([]int64, 0, len(ts))
	tids := make([]int64, 0, len(ts))
	for _, t := range ts {
		cids = append(cids, t.CustomerID)
		tids = append(tids, t.ID)
	}
	customers, err := s.CustomersByID(ctx, cids)
	if err != nil {
		return err
	}
	drafts, err := s.DraftsByTicket(ctx, tids)
	if err != nil {
		return err
	}
	for i := range ts {
		ts[i].Customer = customers[ts[i].CustomerID]
		ts[i].Drafts = drafts[ts[i].ID]
		if ts[i].Drafts == nil {
			ts[i].Drafts = []Draft{}
		}
	}
	return nil
}

const ticketCols = `id, customer_id, subject, body, category, status, created_at`

func scanTicket(row pgx.Row, t *Ticket) error {
	return row.Scan(&t.ID, &t.CustomerID, &t.Subject, &t.Body, &t.Category, &t.Status, &t.CreatedAt)
}

func (s *Store) Tickets(ctx context.Context, status string, limit int) ([]Ticket, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+ticketCols+` FROM tickets
		 WHERE ($1::text = '' OR status = $1::text) ORDER BY created_at DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ticket{}
	for rows.Next() {
		var t Ticket
		if err := scanTicket(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, s.hydrateTickets(ctx, out)
}

func (s *Store) Ticket(ctx context.Context, id int64) (*Ticket, error) {
	var t Ticket
	err := scanTicket(s.Pool.QueryRow(ctx, `SELECT `+ticketCols+` FROM tickets WHERE id = $1`, id), &t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	ts := []Ticket{t}
	if err := s.hydrateTickets(ctx, ts); err != nil {
		return nil, err
	}
	return &ts[0], nil
}

func (s *Store) CreateTicket(ctx context.Context, customerID int64, subject, body, category string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO tickets (customer_id, subject, body, category) VALUES ($1, $2, $3, $4) RETURNING id`,
		customerID, subject, body, category).Scan(&id)
	return id, err
}

func (s *Store) CreateDraft(ctx context.Context, ticketID int64, body string, sources []string) error {
	if sources == nil {
		sources = []string{}
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO ticket_drafts (ticket_id, body, sources) VALUES ($1, $2, $3)`, ticketID, body, sources)
	return err
}

// ReviewDraft approves or rejects a pending draft (optionally editing its text first).
// Approving marks the ticket as replied. Only pending drafts can be reviewed.
func (s *Store) ReviewDraft(ctx context.Context, id int64, approve bool, body *string) (*Draft, error) {
	status := "rejected"
	if approve {
		status = "approved"
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var d Draft
	err = tx.QueryRow(ctx,
		`UPDATE ticket_drafts SET status = $2, body = COALESCE($3::text, body), reviewed_at = now()
		 WHERE id = $1 AND status = 'pending'
		 RETURNING id, ticket_id, body, status, sources, created_at`,
		id, status, body).Scan(&d.ID, &d.TicketID, &d.Body, &d.Status, &d.Sources, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotPending
	}
	if err != nil {
		return nil, err
	}
	if approve {
		if _, err := tx.Exec(ctx, `UPDATE tickets SET status = 'replied' WHERE id = $1`, d.TicketID); err != nil {
			return nil, err
		}
	}
	return &d, tx.Commit(ctx)
}

func (s *Store) Campaigns(ctx context.Context) ([]Campaign, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, subject, status, sent_count, scheduled_for FROM campaigns ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Campaign{}
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.Name, &c.Subject, &c.Status, &c.SentCount, &c.ScheduledFor); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Search implements ai.Retriever using PostgreSQL full-text search (OR across keywords, ranked).
func (s *Store) Search(ctx context.Context, keywords []string, limit int) ([]ai.Article, error) {
	if len(keywords) == 0 {
		return nil, nil
	}
	q := strings.Join(keywords, " | ")
	rows, err := s.Pool.Query(ctx,
		`SELECT title, body FROM kb_articles
		 WHERE search @@ to_tsquery('english', $1)
		 ORDER BY ts_rank(search, to_tsquery('english', $1)) DESC LIMIT $2`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ai.Article
	for rows.Next() {
		var a ai.Article
		if err := rows.Scan(&a.Title, &a.Body); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
