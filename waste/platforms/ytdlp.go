/*
 * ● WasteMusic
 * ○ A high-performance engine for streaming music in Telegram voicechats.
 *
 * Copyright (C) 2026 Waste
 *
 * This program is free software: you can redistribute it and/or modify it under the
 * terms of the GNU General Public License as published by the Free Software Foundation,
 * either version 3 of the License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful, but WITHOUT ANY
 * WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
 * PARTICULAR PURPOSE. See the GNU General Public License for more details.
 *
 * Repository: https://github.com/Oyekanhaa/WasteMusic
 */

package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"WasteMusic/config/cookies"
	"WasteMusic/waste/logger"

	td "github.com/Kanha/Meow"

	state "WasteMusic/waste/core/models"
)

const PlatformYtDlp state.PlatformName = "YtDlp"

type YtdlpPlatform struct{}

type ytdlpInfo struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Duration    float64     `json:"duration"`
	Thumbnail   string      `json:"thumbnail"`
	URL         string      `json:"webpage_url"`
	OriginalURL string      `json:"original_url"`
	Uploader    string      `json:"uploader"`
	IsLive      bool        `json:"is_live"`
	Extractor   string      `json:"extractor"`
	Entries     []ytdlpInfo `json:"entries"`
}

var (
	bannedExtractors = map[string]bool{
		"alphaporno": true, "beeg": true, "behindkink": true, "bongacams": true,
		"cam4": true, "cammodels": true, "camsoda": true, "chaturbate": true,
		"drtuber": true, "eporner": true, "erocast": true, "eroprofile": true,
		"fourtube": true, "goshgay": true, "hellporno": true, "iwara": true,
		"lovehomeporn": true, "manyvids": true, "motherless": true, "murrtube": true,
		"nonktube": true, "noodlemagazine": true, "nubilesporn": true, "nuvid": true,
		"oftv": true, "peekvids": true, "pornbox": true, "pornflip": true,
		"pornhub": true, "pornotube": true, "pornovoisines": true, "pornoxo": true,
		"redgifs": true, "redtube": true, "rule34video": true, "sauceplus": true,
		"sexu": true, "slutload": true, "spankbang": true, "stripchat": true,
		"sunporno": true, "thisvid": true, "tnaflix": true, "toypics": true,
		"txxx": true, "xhamster": true, "xnxx": true, "xvideos": true,
		"xxxymovies": true, "youjizz": true, "youporn": true, "zenporn": true,
	}

	audioOnlyExtractors = map[string]bool{
		"soundcloud": true,
	}

	ytURLPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(youtube\.com|youtu\.be|music\.youtube\.com)`),
	}
)

func init() {
	Register(&YtdlpPlatform{})
}

func (y *YtdlpPlatform) Name() state.PlatformName { return PlatformYtDlp }
func (y *YtdlpPlatform) Priority() int            { return 60 }

func (y *YtdlpPlatform) CanGet(query string) bool {
	if _, err := sanitizeMediaURL(query); err != nil {
		return false
	}

	parsed, err := url.Parse(query)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsed.Host)

	return host != "t.me" &&
		host != "telegram.me" &&
		host != "telegram.dog" &&
		!strings.HasSuffix(host, ".t.me")
}

func (y *YtdlpPlatform) Get(query string, video bool) ([]*state.Track, error) {
	safeURL, err := sanitizeMediaURL(query)
	if err != nil {
		return nil, errUnsafeURL
	}

	info, err := y.extractMetadata(safeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}

	if info.IsLive {
		return nil, errors.New("live streams are not supported")
	}

	if bannedExtractors[strings.ToLower(info.Extractor)] {
		return nil, errors.New("adult content is not allowed")
	}

	var tracks []*state.Track

	if len(info.Entries) > 0 {
		for _, entry := range info.Entries {
			if entry.IsLive ||
				bannedExtractors[strings.ToLower(entry.Extractor)] {
				continue
			}

			tracks = append(tracks, y.toTrack(&entry, video))
		}
	} else {
		tracks = []*state.Track{
			y.toTrack(info, video),
		}
	}

	return tracks, nil
}

// CanDownload keeps YouTube playback independent of any external streaming API.
func (y *YtdlpPlatform) CanDownload(source state.PlatformName) bool {
	return source == PlatformYtDlp || source == PlatformYouTube
}

func ytdlpCookieArgs() ([]string, func()) {
	// yt-dlp may rewrite the cookie jar when it closes. Render Secret Files
	// are read-only, so always give yt-dlp a private writable copy.
	for _, key := range []string{"YTDLP_COOKIES_FILE", "COOKIES_FILE"} {
		if p := strings.TrimSpace(os.Getenv(key)); p != "" {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				logger.Infof("YtDlp: using cookie file from %s: %s", key, p)
				return writableCookieArgs(p)
			}
			logger.Warnf("YtDlp: %s is set but cookie file was not found: %s", key, p)
		}
	}

	if cookieFile, cookieErr := cookies.GetRandomCookieFile(); cookieErr == nil && strings.TrimSpace(cookieFile) != "" {
		if st, err := os.Stat(cookieFile); err == nil && !st.IsDir() {
			logger.Infof("YtDlp: using configured cookie file: %s", cookieFile)
			return writableCookieArgs(cookieFile)
		}
		logger.Warnf("YtDlp: configured cookie path does not exist: %s", cookieFile)
	} else if cookieErr != nil {
		logger.Warnf("YtDlp: cookie manager error: %v", cookieErr)
	}

	matches, _ := filepath.Glob(filepath.Join("config", "cookies", "*.txt"))
	for _, p := range matches {
		if filepath.Base(p) == "example.txt" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			logger.Infof("YtDlp: using local cookie file: %s", p)
			return writableCookieArgs(p)
		}
	}

	logger.Warn("YtDlp: NO YouTube cookie file available")
	return nil, func() {}
}

// writableCookieArgs makes a private writable copy because yt-dlp can save
// cookies during shutdown; mounted secret files must never be passed directly.
func writableCookieArgs(source string) ([]string, func()) {
	in, err := os.Open(source)
	if err != nil {
		logger.Warnf("YtDlp: cannot open cookie file: %v", err)
		return nil, func() {}
	}
	defer in.Close()

	out, err := os.CreateTemp("", "wastemusic-cookies-*.txt")
	if err != nil {
		logger.Warnf("YtDlp: cannot create writable cookie copy: %v", err)
		return nil, func() {}
	}
	path := out.Name()
	if err := out.Chmod(0600); err != nil {
		out.Close()
		os.Remove(path)
		logger.Warnf("YtDlp: cannot secure cookie copy: %v", err)
		return nil, func() {}
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(path)
		logger.Warnf("YtDlp: cannot copy cookie file: %v", err)
		return nil, func() {}
	}
	if err := out.Close(); err != nil {
		os.Remove(path)
		logger.Warnf("YtDlp: cannot close cookie copy: %v", err)
		return nil, func() {}
	}
	return []string{"--cookies", path}, func() { _ = os.Remove(path) }
}

func ytdlpJSRuntimeArgs() []string {
	// Newer yt-dlp YouTube extraction may need a JS runtime and the
	// EJS remote components. Prefer whatever runtime is actually installed.
	for _, rt := range []string{"deno", "node", "bun"} {
		if path, err := exec.LookPath(rt); err == nil {
			args := []string{"--js-runtimes", rt + ":" + path}
			// yt-dlp's EJS components are required by some current
			// YouTube clients. This is harmless when already bundled.
			args = append(args, "--remote-components", "ejs:github")
			return args
		}
	}
	return nil
}

func (y *YtdlpPlatform) Download(
	ctx context.Context,
	track *state.Track,
	_ *td.Message,
) (string, error) {

	if f := findFile(track); f != "" {
		logger.Debug("YtDlp: cache hit " + f)
		return f, nil
	}

	safeURL, err := sanitizeMediaURL(track.URL)
	if err != nil {
		return "", errUnsafeURL
	}

	baseArgs := []string{
		"--no-playlist",
		"--no-part",
		"--geo-bypass",
		"--no-warnings",
		"--no-check-certificate",
		"--retries", "2",
		"--fragment-retries", "2",
		"--socket-timeout", "15",
		"-o", getPath(track, ".%(ext)s"),
	}

	baseArgs = append(baseArgs, ytdlpJSRuntimeArgs()...)

	// YouTube may require authenticated cookies on cloud IPs.
	cookieArgs, cleanupCookies := ytdlpCookieArgs()
	defer cleanupCookies()
	baseArgs = append(baseArgs, cookieArgs...)

	if track.Video {
		baseArgs = append(
			baseArgs,
			"-f",
			"bv*[height<=1080]+ba/b[height<=1080]/b",
		)
	} else {
		baseArgs = append(
			baseArgs,
			"-f", "ba[ext=m4a]/ba/b",
			"-x",
			"--audio-format", "mp3",
			// Quality 5 is a good voice-chat balance and is much faster to encode
			// than VBR quality 0, with no noticeable loss for normal listening.
			"--audio-quality", "5",
			"--concurrent-fragments", "8",
		)
	}

	// YouTube can reject a cloud/Render IP depending on the client profile.
	// Try a few yt-dlp client profiles; none of these require a user API key.
	attempts := make([][]string, 0, 3)
	clientArgs := []string{
		"android,web_safari",
		"tv,web_safari",
		"",
	}
	for _, clients := range clientArgs {
		attempt := append([]string{}, baseArgs...)
		if clients != "" {
			attempt = append(attempt, "--extractor-args", "youtube:player_client="+clients)
		}
		attempts = append(attempts, attempt)
	}

	var lastErr error
	var lastStdout string
	var lastStderr string

	for i, attempt := range attempts {

		findAndRemove(track)

		args := append([]string{}, attempt...)
		args = append(args, "--", safeURL)

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		cmd := exec.CommandContext(
			ctx,
			"yt-dlp",
			args...,
		)

		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()

		lastStdout = strings.TrimSpace(stdout.String())
		lastStderr = strings.TrimSpace(stderr.String())

		if err == nil {

			if p := findFile(track); p != "" {
				logger.Infof(
					"YtDlp: downloaded %s (attempt %d)",
					p,
					i+1,
				)

				return p, nil
			}

			lastErr = errors.New(
				"yt-dlp produced no output file",
			)

			continue
		}

		lastErr = err

		logStderr := lastStderr
		if len(logStderr) > 6000 {
			logStderr = logStderr[len(logStderr)-6000:]
		}

		logStdout := lastStdout
		if len(logStdout) > 3000 {
			logStdout = logStdout[len(logStdout)-3000:]
		}

		logger.Errorf(
			"YtDlp: DOWNLOAD FAILED (attempt %d/%d)\nURL: %s\nCommand error: %v\nSTDERR:\n%s\nSTDOUT:\n%s",
			i+1,
			len(attempts),
			safeURL,
			err,
			logStderr,
			logStdout,
		)

		if isDownloadCancelled(err) {
			return "", err
		}
	}

	findAndRemove(track)

	return "",
		fmt.Errorf(
			"yt-dlp failed after %d attempts: %w\nstdout: %s\nstderr: %s",
			len(attempts),
			lastErr,
			lastStdout,
			lastStderr,
		)
}

func (y *YtdlpPlatform) extractMetadata(
	urlStr string,
) (*ytdlpInfo, error) {

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Minute,
	)
	defer cancel()

	args := []string{
		"-j",
		"--flat-playlist",
		"--no-warnings",
		"--no-check-certificate",
	}
	args = append(args, ytdlpJSRuntimeArgs()...)

	cookieArgs, cleanupCookies := ytdlpCookieArgs()
	defer cleanupCookies()
	args = append(args, cookieArgs...)

	args = append(args,
		"--extractor-args", "youtube:player_client=android,web_safari",
	)

	args = append(args, "--", urlStr)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.CommandContext(
		ctx,
		"yt-dlp",
		args...,
	)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"metadata extraction failed: %w\n%s",
			err,
			stderr.String(),
		)
	}

	lines := strings.Split(
		strings.TrimSpace(stdout.String()),
		"\n",
	)

	if len(lines) > 1 {

		var info ytdlpInfo

		for _, line := range lines {

			var entry ytdlpInfo

			if err := json.Unmarshal(
				[]byte(line),
				&entry,
			); err != nil {

				logger.Debugf(
					"YtDlp: skip bad entry: %v",
					err,
				)

				continue
			}

			info.Entries = append(
				info.Entries,
				entry,
			)
		}

		if len(info.Entries) == 0 {
			return nil, errors.New(
				"no valid entries in playlist",
			)
		}

		return &info, nil
	}

	var info ytdlpInfo

	if err := json.Unmarshal(
		stdout.Bytes(),
		&info,
	); err != nil {

		return nil, fmt.Errorf(
			"failed to parse metadata JSON: %w",
			err,
		)
	}

	return &info, nil
}

func (y *YtdlpPlatform) toTrack(
	info *ytdlpInfo,
	video bool,
) *state.Track {

	if video &&
		audioOnlyExtractors[strings.ToLower(info.Extractor)] {

		video = false
	}

	trackURL := info.URL

	if strings.TrimSpace(info.OriginalURL) != "" {
		trackURL = info.OriginalURL
	}

	return &state.Track{
		ID:       info.ID,
		Title:    info.Title,
		Duration: int(info.Duration),
		Artwork:  info.Thumbnail,
		URL:      trackURL,
		Source:   PlatformYtDlp,
		Video:    video,
	}
}

func (y *YtdlpPlatform) isYouTubeURL(
	urlStr string,
) bool {

	for _, p := range ytURLPatterns {
		if p.MatchString(urlStr) {
			return true
		}
	}

	return false
}
