package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/slh/rift-ctl/internal/cards"
)

const defaultBaseURL = "https://api.riftcodex.com"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient() *Client {
	return &Client{
		BaseURL: defaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) FetchAll(ctx context.Context) ([]cards.Card, error) {
	var out []cards.Card
	page := 1
	const size = 100

	for {
		batch, pages, err := c.fetchPage(ctx, page, size)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		out = append(out, batch...)
		if page >= pages {
			break
		}
		page++
	}
	return out, nil
}

func (c *Client) fetchPage(ctx context.Context, page, size int) ([]cards.Card, int, error) {
	url := fmt.Sprintf("%s/cards?size=%d&page=%d", strings.TrimRight(c.BaseURL, "/"), size, page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, 0, fmt.Errorf("riftcodex %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var pageBody apiPage
	if err := json.NewDecoder(resp.Body).Decode(&pageBody); err != nil {
		return nil, 0, err
	}
	out := make([]cards.Card, 0, len(pageBody.Items))
	for _, raw := range pageBody.Items {
		var api apiCard
		if err := json.Unmarshal(raw, &api); err != nil {
			return nil, 0, err
		}
		card, err := api.toCard(string(raw))
		if err != nil {
			return nil, 0, err
		}
		out = append(out, card)
	}
	return out, pageBody.Pages, nil
}

type apiPage struct {
	Items []json.RawMessage `json:"items"`
	Pages int               `json:"pages"`
}

type apiCard struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	RiftboundID     string            `json:"riftbound_id"`
	TCGPlayerID     *string           `json:"tcgplayer_id"`
	CollectorNumber int               `json:"collector_number"`
	Attributes      apiAttributes     `json:"attributes"`
	Classification  apiClassification `json:"classification"`
	Text            apiText           `json:"text"`
	Set             apiSet            `json:"set"`
	Media           apiMedia          `json:"media"`
	Tags            []string          `json:"tags"`
	Orientation     string            `json:"orientation"`
	Metadata        apiMetadata       `json:"metadata"`
	New             bool              `json:"new"`
}

type apiAttributes struct {
	Energy *int `json:"energy"`
	Might  *int `json:"might"`
	Power  *int `json:"power"`
}

type apiClassification struct {
	Type      string   `json:"type"`
	Supertype *string  `json:"supertype"`
	Rarity    string   `json:"rarity"`
	Domain    []string `json:"domain"`
}

type apiText struct {
	Rich    string  `json:"rich"`
	Plain   string  `json:"plain"`
	Flavour *string `json:"flavour"`
}

type apiSet struct {
	SetID string `json:"set_id"`
	Label string `json:"label"`
}

type apiMedia struct {
	ImageURL          string `json:"image_url"`
	Artist            string `json:"artist"`
	AccessibilityText string `json:"accessibility_text"`
}

type apiMetadata struct {
	CleanName    string  `json:"clean_name"`
	UpdatedOn    *string `json:"updated_on"`
	AlternateArt bool    `json:"alternate_art"`
	Overnumbered bool    `json:"overnumbered"`
	Signature    bool    `json:"signature"`
}

func (a apiCard) toCard(apiJSON string) (cards.Card, error) {
	c := cards.Card{
		ID:                strings.ToLower(a.RiftboundID),
		RiftcodexID:       a.ID,
		Name:              a.Name,
		TCGPlayerID:       deref(a.TCGPlayerID),
		CollectorNumber:   a.CollectorNumber,
		Orientation:       a.Orientation,
		Energy:            a.Attributes.Energy,
		Might:             a.Attributes.Might,
		Power:             a.Attributes.Power,
		Type:              a.Classification.Type,
		Supertype:         deref(a.Classification.Supertype),
		Rarity:            a.Classification.Rarity,
		Domains:           a.Classification.Domain,
		TextPlain:         a.Text.Plain,
		TextRich:          a.Text.Rich,
		Flavour:           deref(a.Text.Flavour),
		SetID:             a.Set.SetID,
		SetLabel:          a.Set.Label,
		ImageURL:          a.Media.ImageURL,
		Artist:            a.Media.Artist,
		AccessibilityText: a.Media.AccessibilityText,
		Tags:              a.Tags,
		CleanName:         a.Metadata.CleanName,
		SourceUpdatedAt:   deref(a.Metadata.UpdatedOn),
		AlternateArt:      a.Metadata.AlternateArt,
		Overnumbered:      a.Metadata.Overnumbered,
		Signature:         a.Metadata.Signature,
		New:               a.New,
		APIJSON:           apiJSON,
		UpdatedAt:         time.Now().UTC(),
	}
	if err := c.Derive(); err != nil {
		return c, err
	}
	return c, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
