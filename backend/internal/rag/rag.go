// Package rag wraps chromem-go, a zero-dependency embeddable vector database.
// It keeps the whole stack to a single binary plus Postgres: there is no
// separate vector service to run. Embeddings are produced by the shared LLM
// client, so dev (mock) and prod (hosted) use the same code path.
package rag

import (
	"context"
	"fmt"

	chromem "github.com/philippgille/chromem-go"

	"github.com/sagun-patwari/ai-career-platform/internal/llm"
)

// Collections we maintain.
const (
	CollectionQuestions = "questions"
	CollectionRoles     = "roles"
)

// Engine is the application-facing handle to the vector store.
type Engine struct {
	db        *chromem.DB
	embedFunc chromem.EmbeddingFunc
}

// Document is a thing we want to retrieve later.
type Document struct {
	ID       string
	Content  string
	Metadata map[string]string
}

// Result is a retrieved document with its similarity score.
type Result struct {
	ID         string
	Content    string
	Metadata   map[string]string
	Similarity float32
}

// New opens (or creates) a persistent vector DB at dir and wires embeddings to
// the LLM client.
func New(dir string, client *llm.Client) (*Engine, error) {
	db, err := chromem.NewPersistentDB(dir, false)
	if err != nil {
		return nil, fmt.Errorf("open vector db: %w", err)
	}
	embedFunc := func(ctx context.Context, text string) ([]float32, error) {
		return client.EmbedOne(ctx, text)
	}
	return &Engine{db: db, embedFunc: embedFunc}, nil
}

func (e *Engine) collection(name string) (*chromem.Collection, error) {
	return e.db.GetOrCreateCollection(name, nil, e.embedFunc)
}

// Upsert adds or replaces documents in a collection.
func (e *Engine) Upsert(ctx context.Context, collection string, docs []Document) error {
	c, err := e.collection(collection)
	if err != nil {
		return err
	}
	cdocs := make([]chromem.Document, 0, len(docs))
	for _, d := range docs {
		cdocs = append(cdocs, chromem.Document{
			ID:       d.ID,
			Content:  d.Content,
			Metadata: d.Metadata,
		})
	}
	if len(cdocs) == 0 {
		return nil
	}
	return c.AddDocuments(ctx, cdocs, 4)
}

// Query returns the n most similar documents to the query text, optionally
// filtered by metadata equality.
func (e *Engine) Query(ctx context.Context, collection, query string, n int, where map[string]string) ([]Result, error) {
	c, err := e.collection(collection)
	if err != nil {
		return nil, err
	}
	count := c.Count()
	if count == 0 {
		return nil, nil
	}
	if n > count {
		n = count
	}
	res, err := c.Query(ctx, query, n, where, nil)
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(res))
	for _, r := range res {
		out = append(out, Result{
			ID:         r.ID,
			Content:    r.Content,
			Metadata:   r.Metadata,
			Similarity: r.Similarity,
		})
	}
	return out, nil
}

// Count returns the number of documents in a collection.
func (e *Engine) Count(collection string) int {
	c, err := e.collection(collection)
	if err != nil {
		return 0
	}
	return c.Count()
}
