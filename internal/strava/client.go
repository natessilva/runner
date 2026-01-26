package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/oauth2"
)

const BaseURL = "https://www.strava.com/api/v3"

// Client is a Strava API client
type Client struct {
	httpClient  *http.Client
	rateLimiter *RateLimiter
}

// NewClient creates a new Strava API client
func NewClient(tokenSource oauth2.TokenSource) *Client {
	return &Client{
		httpClient:  oauth2.NewClient(context.Background(), tokenSource),
		rateLimiter: NewRateLimiter(),
	}
}

// GetActivities fetches activities with pagination
// Returns activities after 'after' timestamp, up to 'perPage' results
func (c *Client) GetActivities(ctx context.Context, after time.Time, page, perPage int) ([]Activity, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	params := url.Values{}
	if !after.IsZero() {
		params.Set("after", strconv.FormatInt(after.Unix(), 10))
	}
	params.Set("page", strconv.Itoa(page))
	params.Set("per_page", strconv.Itoa(perPage))

	resp, err := c.get(ctx, "/athlete/activities", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var activities []Activity
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		return nil, fmt.Errorf("decoding activities: %w", err)
	}

	return activities, nil
}

// GetAllActivities fetches all activities after a given time
// It handles pagination automatically and respects rate limits
func (c *Client) GetAllActivities(ctx context.Context, after time.Time, onProgress func(fetched int)) ([]Activity, error) {
	var allActivities []Activity
	page := 1
	perPage := 100 // Max allowed by Strava

	for {
		activities, err := c.GetActivities(ctx, after, page, perPage)
		if err != nil {
			return allActivities, fmt.Errorf("fetching page %d: %w", page, err)
		}

		if len(activities) == 0 {
			break
		}

		allActivities = append(allActivities, activities...)

		if onProgress != nil {
			onProgress(len(allActivities))
		}

		if len(activities) < perPage {
			break // Last page
		}

		page++
	}

	return allActivities, nil
}

// GetActivityStreams fetches detailed stream data for an activity
func (c *Client) GetActivityStreams(ctx context.Context, activityID int64) (*Streams, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Request all available stream types
	params := url.Values{}
	params.Set("keys", "time,latlng,altitude,velocity_smooth,heartrate,cadence,grade_smooth,distance")
	params.Set("key_by_type", "true")

	path := fmt.Sprintf("/activities/%d/streams", activityID)
	resp, err := c.get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var streams Streams
	if err := json.NewDecoder(resp.Body).Decode(&streams); err != nil {
		return nil, fmt.Errorf("decoding streams: %w", err)
	}

	return &streams, nil
}

// RateLimitStatus returns the current rate limit status
func (c *Client) RateLimitStatus() (shortRemaining, dailyRemaining int) {
	return c.rateLimiter.Status()
}

func (c *Client) get(ctx context.Context, path string, params url.Values) (*http.Response, error) {
	reqURL := BaseURL + path
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Update rate limiter from response headers
	c.rateLimiter.UpdateFromHeaders(resp.Header)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
