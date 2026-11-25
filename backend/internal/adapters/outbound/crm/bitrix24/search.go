package bitrix24

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/437i/client-matcher/backend/internal/domain/client"
	b24 "github.com/bitrix24/b24gosdk"
)

var (
	errConditions = errors.New("bitrix24 search condition budget exceeded")
	errPages      = errors.New("bitrix24 search page budget exceeded")
	errCursor     = errors.New("bitrix24 search cursor did not advance")
	errRows       = errors.New("bitrix24 search returned invalid records")
)

var nameFields = [...]string{titleField, lastField, firstField, patronymicField}

type predicate struct{ field, value string }

func predicates(tokens []client.SearchToken) []predicate {
	var conditions []predicate
	seen := make(map[predicate]bool)
	for _, token := range tokens {
		for _, form := range token.Forms {
			for _, field := range nameFields {
				if form.Kind == client.SearchFormAlias && field != firstField && field != titleField {
					continue
				}
				condition := predicate{field, form.Value}
				if !seen[condition] {
					conditions = append(conditions, condition)
					seen[condition] = true
				}
			}
		}
	}
	return conditions
}

func (r *Repo) Search(ctx context.Context, query string, limit int) ([]client.CRMClient, error) {
	ctx, cancel := context.WithTimeout(ctx, r.policy.searchTimeout)
	defer cancel()
	tokens := client.TokenizeSearchQuery(query)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(tokens) == 0 || limit <= 0 {
		return []client.CRMClient{}, nil
	}
	sourceTokens := client.TokenizeSourceQuery(query)
	if len(sourceTokens) == 0 {
		return []client.CRMClient{}, nil
	}
	records := make(map[string]client.CRMClient)
	remaining := r.policy.maxPages
	if err := r.collect(ctx, predicates(sourceTokens), records, &remaining); err != nil {
		return nil, err
	}
	ranked, err := rank(ctx, tokens, records)
	if err != nil {
		return nil, err
	}
	if r.policy.expandTypos && len(ranked) == 0 && remaining > 0 {
		if extra := typoPredicates(tokens); len(extra) > 0 && len(extra) <= r.policy.maxConditions {
			if err := r.collect(ctx, extra, records, &remaining); err != nil {
				return nil, err
			}
			ranked, err = rank(ctx, tokens, records)
			if err != nil {
				return nil, err
			}
		}
	}
	count := min(limit, len(ranked))
	out := make([]client.CRMClient, 0, count)
	for _, entry := range ranked[:count] {
		out = append(out, entry.record)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func typoPredicates(tokens []client.SearchToken) []predicate {
	var longest []rune
	for _, token := range tokens {
		word := []rune(token.Forms[0].Value)
		if len(word) > len(longest) {
			longest = word
		}
	}
	if len(longest) < 5 {
		return nil
	}
	mid := len(longest) / 2
	return predicates(client.TokenizeName(string(longest[:mid]) + " " + string(longest[mid:])))
}

func (r *Repo) collect(ctx context.Context, conditions []predicate, records map[string]client.CRMClient, remaining *int) error {
	if len(conditions) > r.policy.maxConditions {
		return errConditions
	}
	or := map[string]any{"logic": "OR"}
	for i, condition := range conditions {
		or[strconv.Itoa(i)] = map[string]string{"%" + condition.field: condition.value}
	}
	params := map[string]any{
		"entityTypeId": entityTypeID,
		"select":       []string{"id", titleField, lastField, firstField, patronymicField, birthdayField},
		"order":        map[string]string{"id": "ASC"},
		"filter":       map[string]any{"0": or},
	}
	pager, err := r.sdk.Core().Pages(itemList, params, b24.WithCallOptions(b24.WithTimeout(r.policy.callTimeout)))
	if err != nil {
		return safeSDKError(ctx, err)
	}
	offset := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if *remaining <= 0 {
			return errPages
		}
		if !pager.Next(ctx) {
			if err := pager.Err(); err != nil {
				if errors.Is(err, b24.ErrCursorStalled) {
					return errCursor
				}
				return safeSDKError(ctx, err)
			}
			return errUnavailable
		}
		*remaining = *remaining - 1
		for _, raw := range pager.Rows() {
			if err := ctx.Err(); err != nil {
				return err
			}
			record, err := decodeItem(raw)
			if err != nil {
				return errRows
			}
			if _, exists := records[record.ID]; !exists {
				records[record.ID] = record
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		next := pager.Page().Next
		if next == nil {
			return nil
		}
		if *next <= offset {
			return errCursor
		}
		offset = *next
	}
}

type rankedRecord struct {
	record client.CRMClient
	name   string
	score  int
}

func rank(ctx context.Context, query []client.SearchToken, records map[string]client.CRMClient) ([]rankedRecord, error) {
	ranked := make([]rankedRecord, 0, len(records))
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if score := client.MatchNameTokens(query, client.TokenizeCRMName(record)); score > 0 {
			ranked = append(ranked, rankedRecord{record, client.NormalizeName(record.FullName()), score})
		}
	}
	slices.SortFunc(ranked, func(a, b rankedRecord) int {
		if order := cmp.Compare(b.score, a.score); order != 0 {
			return order
		}
		if order := cmp.Compare(a.name, b.name); order != 0 {
			return order
		}
		return cmp.Compare(a.record.ID, b.record.ID)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return ranked, nil
}
