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

package modules

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	td "github.com/Kanha/Meow"

	"WasteMusic/config"
	"WasteMusic/waste/core"
	"WasteMusic/waste/database"
	"WasteMusic/waste/locales"
)

func sendStartSticker(chatID int64) {
	if config.Token == "" || config.StartStickerID == "" {
		return
	}

	form := url.Values{
		"chat_id": {fmt.Sprintf("%d", chatID)},
		"sticker": {config.StartStickerID},
	}
	req, err := http.NewRequest(http.MethodPost,
		"https://api.telegram.org/bot"+config.Token+"/sendSticker",
		bytes.NewBufferString(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	_, _ = client.Do(req)
}

func startHandler(c *td.Client, m *td.Message) error {
	if isChannel(c, m) {
		return nil
	}

	if !m.IsPrivate() {
		database.AddServedChat(m.ChatID())
		_, err := m.ReplyText(c, F(m.ChatID(), "start_group"), nil)
		return err
	}

	database.AddServedUser(m.ChatID())
	sender, _ := m.GetUser(c)

	if m.Args() == "pm_help" {
		if err := helpHandler(c, m); err != nil {
			return err
		}
	} else if err := handlePmStart(c, m, sender); err != nil {
		return err
	}

	logStart(c, m, sender)
	return nil
}

func startCaption(c *td.Client, chatID int64, sender *td.User, fallbackID int64) string {
	return fmt.Sprintf(
		`𝘏𝘌𝘓𝘓𝘖 𝘑 𝘗 𝘌 𝘟 𝘖 🚩, 𝘔𝘠 𝘕𝘈𝘔𝘌 𝘐𝘚 %s.

𝘐'𝘔 𝘈 𝘛𝘌𝘓𝘌𝘎𝘙𝘈𝘔 𝘚𝘛𝘙𝘌𝘈𝘔𝘐𝘕𝘎 𝘉𝘖𝘛 𝘞𝘐𝘛𝘏 𝘚𝘖𝘔𝘌 𝘜𝘚𝘌𝘍𝘜𝘓 𝘍𝘌𝘈𝘛𝘜𝘙𝘌𝘚. 𝘚𝘜𝘗𝘗𝘖𝘙𝘛𝘐𝘕𝘎 𝘗𝘓𝘈𝘛𝘍𝘖𝘙𝘔𝘚 𝘓𝘐𝘒𝘌 𝘠𝘖𝘜𝘛𝘜𝘉𝘌, 𝘚𝘗𝘖𝘛𝘐𝘍𝘠, 𝘑𝘐𝘖𝘚𝘈𝘈𝘝𝘕, 𝘚𝘖𝘜𝘕𝘋𝘊𝘓𝘖𝘜𝘋 𝘌𝘛𝘊.

𝘍𝘌𝘌𝘓 𝘍𝘙𝘌𝘌 𝘛𝘖 𝘈𝘋𝘋 𝘔𝘌 𝘛𝘖 𝘠𝘖𝘜𝘙 𝘎𝘙𝘖𝘜𝘗𝘚. 👍`,
		config.BotBrand,
	)
}

func handlePmStart(c *td.Client, m *td.Message, sender *td.User) error {
	caption := startCaption(c, m.ChatID(), sender, m.SenderID())

	// Keep the requested first-step sticker before the branded start media.
	sendStartSticker(m.ChatID())

	video := config.StartVideo()
	if video != "" {
		if _, err := c.SendVideo(m.ChatID(), td.InputFileRemote{Id: video}, &td.SendVideoOpts{
			Caption:               caption,
			ParseMode:             td.ParseModeHTML,
			SupportsStreaming:     true,
			ShowCaptionAboveMedia: false,
			ReplyMarkup:           core.GetStartMarkup(m.ChatID()),
		}); err == nil {
			return nil
		}
	}

	if image := config.StartImage(); image != "" {
		if _, err := c.SendPhoto(m.ChatID(), td.InputFileRemote{Id: image}, &td.SendPhotoOpts{
			Caption:     caption,
			ParseMode:   td.ParseModeHTML,
			ReplyMarkup: core.GetStartMarkup(m.ChatID()),
		}); err == nil {
			return nil
		}
	}

	_, err := c.SendTextMessage(m.ChatID(), caption, &td.SendTextMessageOpts{
		ParseMode:   td.ParseModeHTML,
		ReplyMarkup: core.GetStartMarkup(m.ChatID()),
	})
	return err
}

func startCB(c *td.Client, cb *td.UpdateNewCallbackQuery) error {
	cb.Answer(c, 0, false, "", "")

	msg, err := cb.GetMessage(c)
	if err != nil {
		return err
	}
	sender, _ := msg.GetUser(c)

	caption := startCaption(c, cb.ChatId, sender, msg.SenderID())
	markup := core.GetStartMarkup(cb.ChatId)

	if video := config.StartVideo(); video != "" {
		formattedCaption, ferr := c.GetFormattedText(caption, nil, "HTML")
		if ferr != nil {
			return ferr
		}
		content := &td.InputMessageVideo{
			Video: &td.InputVideo{
				Video:             td.InputFileRemote{Id: video},
				SupportsStreaming: true,
			},
			Caption: formattedCaption,
		}
		if _, err = c.EditMessageMedia(cb.ChatId, content, msg.Id, &td.EditMessageMediaOpts{
			ReplyMarkup: markup,
		}); err == nil {
			return nil
		}
	}

	if config.StartImage() == "" {
		_, err = cb.EditMessageText(c, caption, &td.EditTextMessageOpts{
			ReplyMarkup: markup,
			ParseMode:   td.ParseModeHTML,
		})
		return err
	}

	formattedCaption, err := c.GetFormattedText(caption, nil, "HTML")
	if err != nil {
		return err
	}

	content := &td.InputMessagePhoto{
		Photo: &td.InputPhoto{
			Photo: td.InputFileRemote{Id: config.StartImage()},
		},
		Caption: formattedCaption,
	}

	_, err = c.EditMessageMedia(cb.ChatId, content, msg.Id, &td.EditMessageMediaOpts{
		ReplyMarkup: markup,
	})
	return err
}

func isCaptionMediaMessage(m *td.Message) bool {
	if m == nil {
		return false
	}
	switch m.Content.(type) {
	case *td.MessagePhoto, *td.MessageVideo:
		return true
	default:
		return false
	}
}

// isChannelPost reports whether m came from a broadcast channel.
func isChannel(c *td.Client, m *td.Message) bool {
	chat, err := m.GetChat(c)
	if err != nil {
		return false
	}
	sg, ok := chat.Type.(*td.ChatTypeSupergroup)
	return ok && sg.IsChannel
}

// mentionOf builds an HTML mention, falling back to a plain "user" mention
// if sender is nil (e.g. GetUser failed).
func mentionOf(sender *td.User, fallbackId int64) string {
	if sender == nil {
		return mentionOfName("user", fallbackId)
	}
	return mentionOfName(strings.TrimSpace(sender.FirstName+" "+sender.LastName), sender.Id)
}

// mentionOfName builds an HTML mention for a raw name and user ID.
func mentionOfName(name string, id int64) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "user"
	}
	return td.Mention(name, id, true, true)
}

// mentionOfAssistant builds an HTML mention for an assistant account.
func mentionOfAssistant(a *core.Assistant) string {
	if a == nil || a.Self == nil {
		return "Unknown"
	}
	return mentionOfName(strings.TrimSpace(a.Self.FirstName+" "+a.Self.LastName), a.Self.ID)
}

// logStart reports a bot start to the configured logger chat, if enabled.
func logStart(c *td.Client, m *td.Message, sender *td.User) {
	if config.LoggerID == 0 || !isLoggerEnabled() || m.SenderID() == config.OwnerID {
		return
	}

	uName := "N/A"
	if sender != nil && sender.Usernames != nil && len(sender.Usernames.ActiveUsernames) > 0 {
		uName = "@" + sender.Usernames.ActiveUsernames[0]
	}

	msg := F(config.LoggerID, "logger_bot_started", locales.Arg{
		"mention":       mentionOf(sender, m.SenderID()),
		"user_id":       m.SenderID(),
		"user_username": uName,
	})

	if _, err := c.SendTextMessage(config.LoggerID, msg, nil); err != nil {
		c.Logger.Error("Logger send failed", "error", err)
	}
}
