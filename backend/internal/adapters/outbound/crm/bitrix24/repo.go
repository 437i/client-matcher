package bitrix24

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/client"
	"github.com/437i/client-matcher/backend/internal/platform/config"
	b24 "github.com/bitrix24/b24gosdk"
)

const (
	entityTypeID    = 135
	itemGet         = "crm.item.get"
	itemList        = "crm.item.list"
	titleField      = "title"
	lastField       = "ufCrm7_1657182997586"
	firstField      = "ufCrm7_1656048402"
	patronymicField = "ufCrm7_1656048409"
	birthdayField   = "ufCrm7_1656048422"
)

var (
	errUnavailable = errors.New("bitrix24 request failed")
	errQuota       = errors.New("bitrix24 request limit reached")
	errAccess      = errors.New("bitrix24 request denied")
)

type policy struct {
	maxPages, maxConditions    int
	searchTimeout, callTimeout time.Duration
	expandTypos                bool
}

type Repo struct {
	sdk    *b24.Client
	policy policy
}

var _ ports.CRMReader = (*Repo)(nil)

func New(webhook string) (*Repo, error) {
	return newHTTPS(webhook, &http.Client{})
}

func newHTTPS(webhook string, transportClient *http.Client) (*Repo, error) {
	if err := config.ValidateBitrix24WebhookURL(webhook); err != nil {
		return nil, err
	}
	httpClient := *transportClient
	httpClient.Timeout = 3 * time.Second
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return NewWithClient(b24.NewClient(webhook, b24.WithHTTPClient(&httpClient), b24.WithRetry(b24.RetryConfig{MaxAttempts: 1}))), nil
}

func NewWithClient(sdk *b24.Client) *Repo {
	return &Repo{sdk: sdk, policy: policy{maxPages: 20, maxConditions: 64, searchTimeout: 12 * time.Second, callTimeout: 4 * time.Second}}
}

type item struct {
	ID         b24.ID `json:"id"`
	Title      string `json:"title"`
	LastName   string `json:"ufCrm7_1657182997586"`
	FirstName  string `json:"ufCrm7_1656048402"`
	Patronymic string `json:"ufCrm7_1656048409"`
	Birthday   string `json:"ufCrm7_1656048422"`
}

func decodeItem(raw json.RawMessage) (client.CRMClient, error) {
	var dto item
	if err := json.Unmarshal(raw, &dto); err != nil || dto.ID <= 0 {
		return client.CRMClient{}, client.ErrInvalidSourceData
	}
	record := client.CRMClient{ID: dto.ID.String(), SourceFullName: dto.Title, LastName: dto.LastName, FirstName: dto.FirstName, Patronymic: dto.Patronymic}
	if dto.Birthday != "" {
		rawDate := dto.Birthday
		if len(rawDate) != len(time.DateOnly) {
			if _, err := time.Parse(time.RFC3339, rawDate); err != nil || !validTimestampOffset(rawDate) {
				return client.CRMClient{}, client.ErrInvalidSourceData
			}
			rawDate = rawDate[:len(time.DateOnly)]
		}
		date, err := client.ParseDate(rawDate)
		if err != nil {
			return client.CRMClient{}, client.ErrInvalidSourceData
		}
		record.Birthday = &date
	}
	return record, nil
}

func validTimestampOffset(value string) bool {
	if strings.HasSuffix(value, "Z") {
		return true
	}
	offset := value[len(value)-6:]
	// time.Parse accepts offsets whose hours or minutes reach 24 or 60.
	return offset[1:3] <= "23" && offset[4:] <= "59"
}

func (r *Repo) GetByID(ctx context.Context, id string) (client.CRMClient, error) {
	numeric, err := strconv.ParseInt(id, 10, 64)
	if err != nil || numeric <= 0 || strconv.FormatInt(numeric, 10) != id {
		return client.CRMClient{}, client.ErrInvalidCRMID
	}
	raw, err := r.sdk.Core().CallJSON(ctx, itemGet, map[string]any{"entityTypeId": entityTypeID, "id": numeric}, b24.WithIdempotent(), b24.WithTimeout(r.policy.callTimeout))
	if err != nil {
		return client.CRMClient{}, safeSDKError(ctx, err)
	}
	var result struct {
		Item json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return client.CRMClient{}, ctxErr
		}
		return client.CRMClient{}, client.ErrInvalidSourceData
	}
	record, err := decodeItem(result.Item)
	if ctx.Err() != nil {
		return client.CRMClient{}, ctx.Err()
	}
	if err != nil || record.ID != id {
		return client.CRMClient{}, client.ErrInvalidSourceData
	}
	return record, nil
}

func safeSDKError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return context.DeadlineExceeded
	}
	code, _ := b24.CodeOf(err)
	switch strings.ToUpper(string(code)) {
	case "QUERY_LIMIT_EXCEEDED", "OPERATION_TIME_LIMIT":
		return errQuota
	case "ACCESS_DENIED", "INSUFFICIENT_SCOPE":
		return errAccess
	default:
		return errUnavailable
	}
}
