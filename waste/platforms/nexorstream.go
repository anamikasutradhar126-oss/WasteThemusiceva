/*
 * Nexor Stream API progressive downloader.
 * YouTube metadata/search remains handled by the existing YouTube platform.
 * Shruti and yt-dlp remain fallback providers if this API fails.
 */
package platforms

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"WasteMusic/config"
	state "WasteMusic/waste/core/models"
	"WasteMusic/waste/logger"
	td "github.com/Kanha/Meow"
)

const PlatformNexorStream state.PlatformName = "NexorStream"

type NexorStreamPlatform struct{ client *http.Client }

func init() {
	timeout := time.Duration(config.APIStreamTimeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	Register(&NexorStreamPlatform{client: &http.Client{Timeout: timeout}})
}
func (a *NexorStreamPlatform) Name() state.PlatformName { return PlatformNexorStream }
func (a *NexorStreamPlatform) Priority() int            { return 85 }
func (a *NexorStreamPlatform) CanDownload(source state.PlatformName) bool {
	return config.EnableAPI && strings.TrimSpace(config.NexorStreamKey) != "" && source == PlatformYouTube
}
func (a *NexorStreamPlatform) CanGet(_ string) bool { return false }
func (a *NexorStreamPlatform) Get(_ string, _ bool) ([]*state.Track, error) {
	return nil, errors.New("NexorStream is a downloader-only platform")
}
func (a *NexorStreamPlatform) Download(ctx context.Context, track *state.Track, _ *td.Message) (string, error) {
	if track == nil || strings.TrimSpace(track.URL) == "" {
		return "", errors.New("NexorStream: empty track URL")
	}
	if track.Video {
		return "", errors.New("NexorStream supports audio only")
	}
	videoID := extractYouTubeID(track.URL)
	if videoID == "" {
		return "", errors.New("NexorStream: could not extract YouTube video ID")
	}

	base := strings.TrimRight(strings.TrimSpace(config.NexorStreamURL), "/")
	if base == "" {
		return "", errors.New("NexorStream: API URL is not configured")
	}
	u, err := url.Parse(base + "/stream/" + url.PathEscape(videoID))
	if err != nil {
		return "", fmt.Errorf("NexorStream: invalid API URL: %w", err)
	}
	q := u.Query()
	q.Set("key", config.NexorStreamKey)
	u.RawQuery = q.Encode()

	// ZEXXY-style progressive streaming: only wait for the API response headers.
	// Do not download the complete MP3 before starting voice-chat playback.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("NexorStream: create request: %w", err)
	}
	req.Header.Set("Accept", "audio/mpeg,audio/*,application/octet-stream")

	logger.Infof("NexorStream API: opening progressive stream for %s", videoID)
	resp, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("NexorStream API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return "", fmt.Errorf("NexorStream API returned %s: %s", resp.Status, msg)
	}

	ctype := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ctype, "json") || strings.Contains(ctype, "html") || strings.HasPrefix(ctype, "text/") {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("NexorStream API returned non-audio response (%s): %s", ctype, strings.TrimSpace(string(body)))
	}

	// The URL is passed to the existing FFmpeg input pipeline. FFmpeg reads it
	// progressively while Telegram playback starts; the API response above is
	// only a quick availability/content-type check.
	logger.Infof("NexorStream API: progressive stream ready for %s", videoID)
	return u.String(), nil
}
