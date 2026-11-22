/*
 * ArtistBots API downloader.
 *
 * This is the primary YouTube download source. If it is unavailable or
 * returns an invalid response, the platform registry continues to the
 * existing Shruti provider and then yt-dlp (which retains cookie support).
 */
package platforms

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"WasteMusic/config"
	"WasteMusic/waste/logger"

	state "WasteMusic/waste/core/models"
	td "github.com/Kanha/Meow"
)

const PlatformArtistBots state.PlatformName = "ArtistBots"

type ArtistBotsPlatform struct {
	client *http.Client
}

func init() {
	timeout := time.Duration(config.APIStreamTimeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	Register(&ArtistBotsPlatform{
		client: &http.Client{Timeout: timeout},
	})
}

func (a *ArtistBotsPlatform) Name() state.PlatformName { return PlatformArtistBots }
func (a *ArtistBotsPlatform) Priority() int            { return 85 }

func (a *ArtistBotsPlatform) CanDownload(source state.PlatformName) bool {
	return config.EnableAPI &&
		strings.TrimSpace(config.ArtistBotsAPIURL) != "" &&
		strings.TrimSpace(config.ArtistBotsKey) != "" &&
		source == PlatformYouTube
}

func (a *ArtistBotsPlatform) CanGet(_ string) bool { return false }

func (a *ArtistBotsPlatform) Get(_ string, _ bool) ([]*state.Track, error) {
	return nil, errors.New("ArtistBots is a downloader-only platform")
}

func (a *ArtistBotsPlatform) Download(
	ctx context.Context,
	track *state.Track,
	_ *td.Message,
) (string, error) {
	if track == nil || track.URL == "" {
		return "", errors.New("ArtistBots: empty track URL")
	}
	if cached := findFile(track); cached != "" {
		logger.Debug("ArtistBots: cache hit " + cached)
		return cached, nil
	}

	videoID := extractYouTubeID(track.URL)
	if videoID == "" {
		return "", errors.New("ArtistBots: could not extract YouTube video ID")
	}

	kind, ext := "audio", ".mp3"
	if track.Video {
		kind, ext = "video", ".mp4"
	}
	if err := os.MkdirAll("downloads", 0755); err != nil {
		return "", fmt.Errorf("ArtistBots: create downloads directory: %w", err)
	}

	out := getPath(track, ext)
	tmp := out + ".part"
	_ = os.Remove(tmp)

	base := strings.TrimRight(strings.TrimSpace(config.ArtistBotsAPIURL), "/")
	u, err := url.Parse(base + "/download")
	if err != nil {
		return "", fmt.Errorf("ArtistBots: invalid API URL: %w", err)
	}
	q := u.Query()
	q.Set("url", videoID)
	q.Set("type", kind)
	q.Set("api_key", config.ArtistBotsKey)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("ArtistBots: create request: %w", err)
	}
	req.Header.Set("Accept", "audio/*,video/*,application/octet-stream")

	logger.Infof("ArtistBots API: requesting %s (%s)", videoID, kind)
	resp, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("ArtistBots API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return "", fmt.Errorf("ArtistBots API returned %s: %s", resp.Status, msg)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "html") ||
		strings.HasPrefix(contentType, "text/") {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("ArtistBots API returned non-media response (%s): %s",
			contentType, strings.TrimSpace(string(body)))
	}

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("ArtistBots: create temporary file: %w", err)
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("ArtistBots API download failed: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("ArtistBots: close downloaded file: %w", closeErr)
	}

	info, err := os.Stat(tmp)
	if err != nil || info.Size() < 1024 {
		_ = os.Remove(tmp)
		return "", errors.New("ArtistBots API returned an empty or invalid media file")
	}

	// Reject JSON/HTML/text error pages even if the API incorrectly says 200.
	check, err := os.Open(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("ArtistBots: validate downloaded file: %w", err)
	}
	head := make([]byte, 512)
	n, _ := check.Read(head)
	_ = check.Close()
	head = []byte(strings.TrimSpace(string(head[:n])))
	if len(head) > 0 && (head[0] == '{' || head[0] == '[' || head[0] == '<' ||
		strings.HasPrefix(strings.ToLower(string(head)), "error")) {
		_ = os.Remove(tmp)
		return "", errors.New("ArtistBots API returned a text/JSON error instead of media")
	}

	if cached := findFile(track); cached != "" {
		_ = os.Remove(tmp)
		return cached, nil
	}
	_ = os.Remove(out)
	if err := os.Rename(tmp, out); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("ArtistBots: finalize downloaded file: %w", err)
	}

	logger.Infof("ArtistBots API: downloaded %s", out)
	return out, nil
}
