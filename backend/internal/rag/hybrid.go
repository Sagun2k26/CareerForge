package rag

import (
	"math"
	"sort"
	"strings"
)

func lexicalScore(query, doc string) float64 {
	q := tokens(query)
	d := tokens(doc)
	if len(q) == 0 || len(d) == 0 {
		return 0
	}
	freq := map[string]int{}
	for _, t := range d {
		freq[t]++
	}
	var score float64
	for _, t := range q {
		if n := freq[t]; n > 0 {
			score += 1 + math.Log(1+float64(n))
		}
	}
	return score / math.Sqrt(float64(len(d)))
}

func tokens(s string) []string {
	s = strings.ToLower(s)
	f := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '+' || r == '#')
	})
	stop := map[string]bool{"the": true, "a": true, "an": true, "of": true, "to": true, "and": true, "for": true, "in": true, "is": true, "are": true, "how": true, "what": true}
	out := make([]string, 0, len(f))
	for _, x := range f {
		if len(x) > 1 && !stop[x] {
			out = append(out, x)
		}
	}
	return out
}

func dedupeResults(in []Result) []Result {
	seenID := map[string]bool{}
	seenText := map[string]bool{}
	out := make([]Result, 0, len(in))
	for _, r := range in {
		k := strings.Join(tokens(r.Content), " ")
		if seenID[r.ID] || seenText[k] {
			continue
		}
		seenID[r.ID] = true
		seenText[k] = true
		out = append(out, r)
	}
	return out
}

func hybridRerank(query string, in []Result, where map[string]string) []Result {
	type scored struct {
		r Result
		s float64
	}
	a := make([]scored, 0, len(in))
	for _, r := range in {
		sem := float64(r.Similarity)
		lex := lexicalScore(query, r.Content)
		meta := 0.0
		for k, v := range where {
			if strings.EqualFold(r.Metadata[k], v) {
				meta += .08
			}
		}
		a = append(a, scored{r: r, s: .68*sem + .27*lex + .05*meta})
	}
	sort.SliceStable(a, func(i, j int) bool { return a[i].s > a[j].s })
	out := make([]Result, 0, len(a))
	for _, x := range a {
		out = append(out, x.r)
	}
	return dedupeResults(out)
}
