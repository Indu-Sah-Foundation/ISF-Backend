package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	endpoint string
	key      string
	region   string
	http     *http.Client
}

func NewClient(endpoint, key, region string) *Client {
	return &Client{
		endpoint: endpoint,
		key:      key,
		region:   region,
		// Longer articles (the markdown→HTML expanded body) can take
		// 10–20 s end-to-end with Azure Translator. The previous 10 s
		// cap was timing out before the response landed, which meant
		// the SaveTranslation step never ran — so the DB stayed empty.
		http: &http.Client{Timeout: 45 * time.Second},
	}
}

type translateInput struct {
	Text string `json:"text"`
}

type translateOutput struct {
	Translations []struct {
		Text string `json:"text"`
		To   string `json:"to"`
	} `json:"translations"`
}

func (c *Client) Translate(ctx context.Context, texts []string, targetLang string) ([]string, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body := make([]translateInput, len(texts))
	for i, t := range texts {
		body[i] = translateInput{Text: t}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal translate body: %w", err)
	}
	u, err := url.Parse(c.endpoint + "/translate")
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	q.Set("api-version", "3.0")
	q.Set("to", targetLang)
	q.Set("textType", "html")
	u.RawQuery = q.Encode()

	// Retry up to 3x with exponential backoff. Azure Translator
	// throttles aggressively on the free tier (429), and we'd rather
	// the user wait an extra second than see a 500.
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 800 * time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Ocp-Apim-Subscription-Key", c.key)
		req.Header.Set("Ocp-Apim-Subscription-Region", c.region)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("translate http: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var out []translateOutput
			derr := json.NewDecoder(resp.Body).Decode(&out)
			resp.Body.Close()
			if derr != nil {
				return nil, fmt.Errorf("decode translate response: %w", derr)
			}
			if len(out) != len(texts) {
				return nil, fmt.Errorf("translate: expected %d results, got %d", len(texts), len(out))
			}
			result := make([]string, len(texts))
			for i, item := range out {
				if len(item.Translations) == 0 {
					return nil, fmt.Errorf("translate: empty result for index %d", i)
				}
				result[i] = item.Translations[0].Text
			}
			return result, nil
		}

		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastErr = fmt.Errorf("translate api: %d (lang=%s): %s", resp.StatusCode, targetLang, string(b))

		// Retry only on transient codes; everything else is a permanent
		// failure (unsupported language, malformed body, bad key).
		if resp.StatusCode != http.StatusTooManyRequests &&
			resp.StatusCode < http.StatusInternalServerError {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

type Language struct {
	Name       string `json:"name"`
	NativeName string `json:"nativeName"`
	Dir        string `json:"dir"`
}

type languageResponse struct {
	Translation map[string]Language `json:"translation"`
}

func (c *Client) Languages(ctx context.Context) (map[string]Language, error) {
	u := c.endpoint + "/languages?api-version=3.0&scope=translation"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("languages http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("languages api: %d: %s", resp.StatusCode, string(b))
	}
	var out languageResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode languages: %w", err)
	}
	return out.Translation, nil
}
