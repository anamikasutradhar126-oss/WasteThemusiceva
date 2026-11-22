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
	"fmt"
	"html"
	"strings"

	td "github.com/Kanha/Meow"

	"WasteMusic/config"
	"WasteMusic/waste/core"
	"WasteMusic/waste/utils"
)

var helpTexts = map[string]string{}

// smallCapsHTML converts the visible help-menu text to the small-caps style
// used in the requested design while preserving Telegram HTML tags/entities.
func smallCapsHTML(s string) string {
	// The existing English locale uses mathematical italic Unicode letters.
	// Convert those back to normal Latin first so they can be rendered in the
	// requested small-caps alphabet.
	var out strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); {
		if r[i] == '<' {
			j := i
			for j < len(r) && r[j] != '>' {
				j++
			}
			if j < len(r) {
				out.WriteString(string(r[i : j+1]))
				i = j + 1
				continue
			}
		}
		if r[i] == '&' {
			j := i
			for j < len(r) && r[j] != ';' && j-i < 12 {
				j++
			}
			if j < len(r) && r[j] == ';' {
				out.WriteString(string(r[i : j+1]))
				i = j + 1
				continue
			}
		}

		ch := r[i]
		// Mathematical Sans/Italic capital and small letters used by the old
		// help_main locale string.
		switch {
		case ch >= 0x1D608 && ch <= 0x1D621:
			ch = 'A' + (ch - 0x1D608)
		case ch >= 0x1D622 && ch <= 0x1D63B:
			ch = 'a' + (ch - 0x1D622)
		}

		switch ch {
		case 'A', 'a':
			out.WriteRune('ᴀ')
		case 'B', 'b':
			out.WriteRune('ʙ')
		case 'C', 'c':
			out.WriteRune('ᴄ')
		case 'D', 'd':
			out.WriteRune('ᴅ')
		case 'E', 'e':
			out.WriteRune('ᴇ')
		case 'F', 'f':
			out.WriteRune('ғ')
		case 'G', 'g':
			out.WriteRune('ɢ')
		case 'H', 'h':
			out.WriteRune('ʜ')
		case 'I', 'i':
			out.WriteRune('ɪ')
		case 'J', 'j':
			out.WriteRune('ᴊ')
		case 'K', 'k':
			out.WriteRune('ᴋ')
		case 'L', 'l':
			out.WriteRune('ʟ')
		case 'M', 'm':
			out.WriteRune('ᴍ')
		case 'N', 'n':
			out.WriteRune('ɴ')
		case 'O', 'o':
			out.WriteRune('ᴏ')
		case 'P', 'p':
			out.WriteRune('ᴘ')
		case 'Q', 'q':
			out.WriteRune('ǫ')
		case 'R', 'r':
			out.WriteRune('ʀ')
		case 'S', 's':
			out.WriteRune('s')
		case 'T', 't':
			out.WriteRune('ᴛ')
		case 'U', 'u':
			out.WriteRune('ᴜ')
		case 'V', 'v':
			out.WriteRune('ᴠ')
		case 'W', 'w':
			out.WriteRune('ᴡ')
		case 'X', 'x':
			out.WriteRune('x')
		case 'Y', 'y':
			out.WriteRune('ʏ')
		case 'Z', 'z':
			out.WriteRune('ᴢ')
		default:
			out.WriteRune(ch)
		}
		i++
	}
	return out.String()
}

func formatHelpMenu(chatID int64) string {
	text := smallCapsHTML(F(chatID, "help_main"))
	// The requested design is plain small-caps text, not the old italic/bold
	// mathematical font. Keep the Telegram blockquote around the whole message.
	text = strings.ReplaceAll(text, "<b>", "")
	text = strings.ReplaceAll(text, "</b>", "")
	text = strings.ReplaceAll(text, "<i>", "")
	text = strings.ReplaceAll(text, "</i>", "")
	return "<blockquote>" + text + "</blockquote>"
}

func init() {
	helpTexts["/help"] = fmt.Sprintf(`ℹ️ <b>Help Command</b>
<i>Displays general bot help or detailed information about a specific command.</i>

<u>Usage:</u>
<code>/help</code> — Show the main help menu.  
<code>/help &lt;command&gt;</code> — Show help for a specific command.

For more info, visit our <a href="%s">Support Chat</a>.`, config.SupportChat)
}

func helpHandler(c *td.Client, m *td.Message) error {
	if isChannel(c, m) {
		return nil
	}

	args := strings.Fields(m.Text())
	if len(args) > 1 {
		cmd := args[1]
		if cmd != "pm_help" {
			if !strings.HasPrefix(cmd, "/") {
				cmd = "/" + cmd
			}
			return showHelpFor(c, m, cmd)
		}
	}

	if !m.IsPrivate() {
		_, err := m.ReplyText(c, F(m.ChatID(), "help_private_only"), &td.SendTextMessageOpts{
			ReplyMarkup: core.GetGroupHelpKeyboard(m.ChatID()),
		})
		return err
	}

	caption := formatHelpMenu(m.ChatID())
	markup := core.GetHelpKeyboard(m.ChatID())

	if video := config.HelpVideo(); video != "" {
		if _, err := c.SendVideo(m.ChatID(), td.InputFileRemote{Id: video}, &td.SendVideoOpts{
			Caption:           caption,
			ParseMode:         td.ParseModeHTML,
			SupportsStreaming: true,
			ReplyMarkup:       markup,
		}); err == nil {
			return nil
		}
	}

	if image := config.StartImage(); image != "" {
		if _, err := c.SendPhoto(m.ChatID(), td.InputFileRemote{Id: image}, &td.SendPhotoOpts{
			Caption:     caption,
			ParseMode:   td.ParseModeHTML,
			ReplyMarkup: markup,
		}); err == nil {
			return nil
		}
	}

	_, err := m.ReplyText(c, caption, &td.SendTextMessageOpts{
		ParseMode:   td.ParseModeHTML,
		ReplyMarkup: markup,
	})
	return err
}

func helpCB(c *td.Client, cb *td.UpdateNewCallbackQuery) error {
	cb.Answer(c, 0, false, "", "")

	msg, err := cb.GetMessage(c)
	if err != nil {
		return err
	}

	markup := core.GetHelpKeyboard(cb.ChatId)

	if isCaptionMediaMessage(msg) {
		_, err = cb.EditMessageCaption(c, formatHelpMenu(cb.ChatId), &td.EditCaptionOpts{
			ReplyMarkup: markup,
		})
	} else {
		_, err = cb.EditMessageText(c, formatHelpMenu(cb.ChatId), &td.EditTextMessageOpts{
			ReplyMarkup: markup,
		})
	}
	return err
}

func helpCallbackHandler(c *td.Client, cb *td.UpdateNewCallbackQuery) error {
	data := cb.DataString()
	cb.Answer(c, 0, false, "", "")
	if data == "" {
		return nil
	}
	chatID := cb.ChatId
	parts := strings.SplitN(data, ":", 2)
	if len(parts) < 2 {
		return nil
	}

	var text string
	btn := core.GetBackKeyboard(chatID)

	switch parts[1] {
	case "admin":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_admin")) + "</blockquote>"
	case "auth":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_auth")) + "</blockquote>"
	case "bcast":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_bcast")) + "</blockquote>"
	case "play":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_play")) + "</blockquote>"
	case "sudo":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_sudo")) + "</blockquote>"
	case "restrict":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_restrict")) + "</blockquote>"
	case "thumb":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_thumb")) + "</blockquote>"
	case "start":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_start")) + "</blockquote>"
	case "autoplay":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_autoplay")) + "</blockquote>"
	case "playlist":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_playlist")) + "</blockquote>"
	case "vclogs":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_vclogs")) + "</blockquote>"
	case "inline":
		text = "<blockquote>" + utils.StylizeHTML(F(chatID, "help_inline")) + "</blockquote>"
	case "main":
		text = formatHelpMenu(chatID)
		btn = core.GetHelpKeyboard(chatID)
	}

	msg, err := cb.GetMessage(c)
	if err != nil {
		return err
	}

	if isCaptionMediaMessage(msg) {
		_, err = cb.EditMessageCaption(c, text, &td.EditCaptionOpts{
			ReplyMarkup: btn,
		})
	} else {
		_, err = cb.EditMessageText(c, text, &td.EditTextMessageOpts{
			ReplyMarkup: btn,
		})
	}
	return err
}

func showHelpFor(c *td.Client, m *td.Message, cmd string) error {
	helpText, ok := helpTexts[cmd]
	if !ok {
		trimmed := strings.TrimPrefix(cmd, "/")
		if value, exists := helpTexts[trimmed]; exists {
			helpText = value
		}
	}

	if helpText == "" {
		_, err := m.ReplyText(
			c,
			"⚠️ <i>No help found for command <code>"+
				html.EscapeString(cmd)+
				"</code></i>",
			nil,
		)
		return err
	}

	helpText = "<blockquote>" + utils.StylizeHTML(helpText) + "</blockquote>"
	_, err := m.ReplyText(
		c,
		"📘 <b>Help for</b> <code>"+cmd+"</code>:\n\n"+helpText,
		nil,
	)
	return err
}
