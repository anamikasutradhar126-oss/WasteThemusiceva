/*
 * Nexor Stream API downloader.
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
	"os"
	"path/filepath"
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
	if cached := findFile(track); cached != "" {
		logger.Debug("NexorStream cache hit " + cached)
		return cached, nil
	}
	videoID := extractYouTubeID(track.URL)
	if videoID == "" {
		return "", errors.New("NexorStream: could not extract YouTube video ID")
	}
	if err := os.MkdirAll("downloads", 0755); err != nil {
		return "", fmt.Errorf("NexorStream: create downloads directory: %w", err)
	}
	out := filepath.Join("downloads", "audio_"+videoID+".mp3")
	tmp := out + ".part"
	_ = os.Remove(tmp)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("NexorStream: create request: %w", err)
	}
	req.Header.Set("Accept", "audio/mpeg,audio/*,application/octet-stream")
	logger.Infof("NexorStream API: requesting %s", videoID)
	resp, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("NexorStream API request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
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
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("NexorStream: create temp file: %w", err)
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("NexorStream stream download failed: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("NexorStream close file: %w", closeErr)
	}
	info, err := os.Stat(tmp)
	if err != nil || info.Size() < 1024 {
		_ = os.Remove(tmp)
		return "", errors.New("NexorStream returned empty/invalid audio")
	}
	check, err := os.Open(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("NexorStream validate audio: %w", err)
	}
	head := make([]byte, 512)
	n, _ := check.Read(head)
	_ = check.Close()
	head = []byte(strings.TrimSpace(string(head[:n])))
	if len(head) > 0 && (head[0] == '{' || head[0] == '[' || head[0] == '<' || strings.HasPrefix(strings.ToLower(string(head)), "error")) {
		_ = os.Remove(tmp)
		return "", errors.New("NexorStream returned text/JSON instead of audio")
	}
	_ = os.Remove(out)
	if err := os.Rename(tmp, out); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("NexorStream finalize audio: %w", err)
	}
	if info, statErr := os.Stat(out); statErr != nil || !info.Mode().IsRegular() || info.Size() < 1024 {
		_ = os.Remove(out)
		return "", errors.New("NexorStream finalized audio is invalid")
	}
	logger.Infof("NexorStream API: downloaded audio %s", out)
	return out, nil
}
