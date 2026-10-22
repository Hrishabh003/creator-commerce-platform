// Package ai implements the support workflow: categorize a ticket, retrieve
// knowledge-base context, and draft a reply. Drafts are never sent automatically;
// an operator approves or rejects them.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Article struct {
	Title string
	Body  string
}

type DraftInput struct {
	CustomerName string
	Subject      string
	Body         string
	Category     string
	Articles     []Article
}

type Retriever interface {
	Search(ctx context.Context, keywords []string, limit int) ([]Article, error)
}

type Drafter interface {
	Draft(ctx context.Context, in DraftInput) (string, error)
}

// Result is what the pipeline hands back to the API layer.
type Result struct {
	Category string
	Draft    string
	Sources  []string
}

type Pipeline struct {
	Retriever Retriever
	Drafter   Drafter
}

// Categorize returns the first category with the most keyword hits, or "general".
func Categorize(text string) string {
	counts := map[string]int{}
	for _, t := range tokenize(text) {
		counts[t]++
	}
	best, bestScore := "general", 0
	for _, r := range rules {
		score := 0
		for _, w := range r.words {
			score += counts[w]
		}
		if score > bestScore {
			best, bestScore = r.category, score
		}
	}
	return best
}

var rules = []struct {
	category string
	words    []string
}{
	{"billing", []string{"refund", "charge", "charged", "invoice", "payment", "billing", "receipt"}},
	{"shipping", []string{"shipping", "delivery", "delivered", "tracking", "package", "arrive", "arrived", "late"}},
	{"returns", []string{"return", "exchange", "damaged", "broken", "wrong"}},
	{"product", []string{"size", "material", "color", "colour", "stock", "design"}},
	{"account", []string{"password", "login", "account", "email", "address"}},
}

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

func tokenize(s string) []string { return wordRe.FindAllString(strings.ToLower(s), -1) }

var stop = map[string]bool{
	"this": true, "that": true, "with": true, "have": true, "what": true, "where": true,
	"there": true, "about": true, "would": true, "could": true, "please": true, "thanks": true,
	"hello": true, "from": true, "your": true, "been": true, "they": true, "them": true,
	"does": true, "into": true, "just": true, "when": true,
}

// Keywords returns up to 8 distinct, lowercase, alphanumeric search terms.
func Keywords(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tokenize(text) {
		if len(t) <= 3 || stop[t] || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) == 8 {
			break
		}
	}
	return out
}

// Process runs categorize -> retrieve -> draft.
func (p *Pipeline) Process(ctx context.Context, customerName, subject, body string) (Result, error) {
	text := subject + " " + body
	category := Categorize(text)
	articles, err := p.Retriever.Search(ctx, Keywords(text), 3)
	if err != nil {
		return Result{Category: category}, err
	}
	draft, err := p.Drafter.Draft(ctx, DraftInput{
		CustomerName: customerName, Subject: subject, Body: body,
		Category: category, Articles: articles,
	})
	if err != nil {
		return Result{Category: category}, err
	}
	sources := make([]string, 0, len(articles))
	for _, a := range articles {
		sources = append(sources, a.Title)
	}
	return Result{Category: category, Draft: draft, Sources: sources}, nil
}

// TemplateDrafter needs no API key. It quotes the first sentence of each retrieved article.
type TemplateDrafter struct{}

func (TemplateDrafter) Draft(_ context.Context, in DraftInput) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Hi %s,\n\n", firstName(in.CustomerName))
	if len(in.Articles) == 0 {
		b.WriteString("Thanks for your message. I've passed it to a teammate who will follow up shortly.\n")
	} else {
		fmt.Fprintf(&b, "Thanks for reaching out about %s. Here is what we found:\n\n", in.Category)
		for _, a := range in.Articles {
			fmt.Fprintf(&b, "- %s\n", firstSentence(a.Body))
		}
		b.WriteString("\nIf this doesn't resolve it, reply here and a teammate will follow up.\n")
	}
	b.WriteString("\nSupport team")
	return b.String(), nil
}

func firstName(n string) string {
	if f := strings.Fields(n); len(f) > 0 {
		return f[0]
	}
	return "there"
}

func firstSentence(s string) string {
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i+1]
	}
	return strings.TrimSpace(s)
}

// ClaudeDrafter asks the Anthropic Messages API for a draft, grounded in retrieved articles.
type ClaudeDrafter struct {
	apiKey, model string
	client        *http.Client
}

func NewClaudeDrafter(apiKey, model string) *ClaudeDrafter {
	return &ClaudeDrafter{apiKey: apiKey, model: model, client: &http.Client{Timeout: 20 * time.Second}}
}

const systemPrompt = "You draft customer support replies for a commerce operator to review. " +
	"Use ONLY the knowledge-base context provided. If it does not answer the question, say a teammate will follow up. " +
	"Be concise and friendly. Never promise refunds, dates, or policies that are not in the context."

func (c *ClaudeDrafter) Draft(ctx context.Context, in DraftInput) (string, error) {
	var kb strings.Builder
	for i, a := range in.Articles {
		fmt.Fprintf(&kb, "[%d] %s: %s\n", i+1, a.Title, a.Body)
	}
	user := fmt.Sprintf("Customer: %s\nCategory: %s\nSubject: %s\nMessage: %s\n\nKnowledge base:\n%s\nWrite the reply.",
		in.CustomerName, in.Category, in.Subject, in.Body, kb.String())
	payload, err := json.Marshal(map[string]any{
		"model":      c.model,
		"max_tokens": 500,
		"system":     systemPrompt,
		"messages":   []map[string]string{{"role": "user", "content": user}},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic: unexpected status %d", resp.StatusCode)
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	for _, blk := range out.Content {
		if blk.Type == "text" && strings.TrimSpace(blk.Text) != "" {
			return strings.TrimSpace(blk.Text), nil
		}
	}
	return "", fmt.Errorf("anthropic: empty response")
}

// FallbackDrafter uses Backup when Primary fails, so a provider outage never blocks ticket intake.
type FallbackDrafter struct{ Primary, Backup Drafter }

func (f FallbackDrafter) Draft(ctx context.Context, in DraftInput) (string, error) {
	if out, err := f.Primary.Draft(ctx, in); err == nil {
		return out, nil
	}
	return f.Backup.Draft(ctx, in)
}
