// Package rag wraps chromem-go, a zero-dependency embeddable vector database.
package rag

import (
	"context"
	"fmt"

	chromem "github.com/philippgille/chromem-go"

	"github.com/sagun-patwari/ai-career-platform/internal/llm"
)

const (
	CollectionQuestions = "questions"
	CollectionRoles     = "roles"
)

type Engine struct {
	db        *chromem.DB
	embedFunc chromem.EmbeddingFunc
}

type Document struct {
	ID       string
	Content  string
	Metadata map[string]string
}

type Result struct {
	ID         string
	Content    string
	Metadata   map[string]string
	Similarity float32
}

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

// queryEmbedding safely contains a chromem-go panic that occurs when n is
// larger than the number of documents left after metadata filtering.
func queryEmbedding(
	ctx context.Context,
	c *chromem.Collection,
	embedding []float32,
	n int,
	where map[string]string,
) (res []chromem.Result, panicked bool, err error) {
	defer func() {
		if recover() != nil {
			res = nil
			panicked = true
			err = nil
		}
	}()
	res, err = c.QueryEmbedding(ctx, embedding, n, where, nil)
	return res, false, err
}

// Query retrieves a larger semantic candidate set, safely handles metadata
// filters that leave fewer documents than requested, then hybrid-reranks and
// truncates back to the caller's requested top-K.
func (e *Engine) Query(ctx context.Context, collection, query string, n int, where map[string]string) ([]Result, error) {
	c, err := e.collection(collection)
	if err != nil {
		return nil, err
	}

	count := c.Count()
	if count == 0 || n <= 0 {
		return nil, nil
	}
	if n > count {
		n = count
	}

	requested := n

	// Overfetch semantic candidates so lexical relevance + metadata reranking
	// has enough candidates to improve ordering.
	candidateK := n * 4
	if candidateK < 20 {
		candidateK = 20
	}
	if candidateK > count {
		candidateK = count
	}

	embedding, err := e.embedFunc(ctx, query)
	if err != nil {
		return nil, err
	}

	// Find the largest safe K up to candidateK. chromem-go can panic when a
	// metadata filter leaves fewer matching documents than K.
	low, high := 1, candidateK
	var best []chromem.Result

	for low <= high {
		mid := (low + high) / 2

		res, panicked, qerr := queryEmbedding(ctx, c, embedding, mid, where)
		if qerr != nil {
			return nil, qerr
		}
		if panicked {
			high = mid - 1
			continue
		}

		best = res
		low = mid + 1
	}

	if len(best) == 0 {
		return nil, nil
	}

	out := make([]Result, 0, len(best))
	for _, r := range best {
		out = append(out, Result{
			ID:         r.ID,
			Content:    r.Content,
			Metadata:   r.Metadata,
			Similarity: r.Similarity,
		})
	}

	// semantic similarity + lexical overlap + metadata relevance + dedup
	out = hybridRerank(query, out, where)

	if len(out) > requested {
		out = out[:requested]
	}

	return out, nil
}

func (e *Engine) Count(collection string) int {
	c, err := e.collection(collection)
	if err != nil {
		return 0
	}
	return c.Count()
}
