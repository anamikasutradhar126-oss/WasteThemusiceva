/*
 * WASTE X MUSIC — group welcome / join-request flow.
 *
 * Keeps the existing music commands untouched and adds only the branded
 * onboarding media/messages requested for the bot.
 */

package modules

import (
	"fmt"
	"strings"

	td "github.com/Kanha/Meow"

	"WasteMusic/config"
	"WasteMusic/waste/core"
	"WasteMusic/waste/database"
	"WasteMusic/waste/utils"
)

const welcomeCaption = `<blockquote>𐙚 ᴡᴇʟᴄᴏᴍᴇ ǫᴛ / ǫᴛᴀ {chatname}

♡ ❤️
🤩 ɴᴀᴍᴇ : {fullname}
🩷ɢʀᴏᴜᴘ : {chatname}
😊 ᴜsᴇʀɴᴀᴍᴇ : {username}
💗 ᴜsᴇʀ ɪᴅ : {id}

୨୧ ᴛʜᴀɴᴋ ʏᴏᴜ ғᴏʀ ᴊᴏɪɴɪɴɢ ᴏᴜʀ ʟɪᴛᴛʟᴇ ᴡᴏʀʟᴅ! 💐</blockquote>`

const addedCaption = `𝘏𝘦𝘺 {bot}, 👋

𝘞𝘦𝘭𝘤𝘰𝘮𝘦 𝘵𝘰 {group}!

𝘛𝘩𝘪𝘴 𝘐𝘴 {brand}.

𝘛𝘩𝘢𝘯𝘬𝘴 𝘍𝘰𝘳 𝘈𝘥𝘥𝘪𝘯𝘨 𝘈𝘐 𝘔𝘶𝘴𝘪𝘤 𝘛𝘦𝘴𝘵𝘪𝘯𝘨, {brand} 𝘊𝘢𝘯 𝘗𝘭𝘢𝘺 𝘚𝘰𝘯𝘨𝘴 𝘐𝘯 𝘛𝘩𝘪𝘴 𝘊𝘩𝘢𝘵.

𝘏𝘦𝘺 {user}, 𝘵𝘩𝘢𝘯𝘬𝘴 𝘧𝘰𝘳 𝘢𝘥𝘥𝘪𝘯𝘨 𝘮𝘦!`

func welcomeToggleHandler(c *td.Client, m *td.Message) error {
	if m == nil || m.IsPrivate() || isChannel(c, m) {
		return nil
	}
	args := strings.Fields(m.Text())
	if len(args) < 2 {
		enabled, _ := database.IsWelcomeEnabled(m.ChatID())
		state := "ᴇɴᴀʙʟᴇᴅ"
		if !enabled {
			state = "ᴅɪsᴀʙʟᴇᴅ"
		}
		_, err := m.ReplyText(c,
			fmt.Sprintf("<blockquote>𝘞𝘦𝘭𝘤𝘰𝘮𝘦 𝘮𝘦𝘴𝘴𝘢𝘨𝘦𝘴: <b>%s</b>\n\n𝘜𝘴𝘢𝘨𝘦:\n<code>/welcome on</code>\n<code>/welcome off</code></blockquote>", state),
			&td.SendTextMessageOpts{ParseMode: td.ParseModeHTML})
		return err
	}

	isAdmin, _ := utils.IsChatAdmin(c, m.ChatID(), m.SenderID())
	if !isAdmin && m.SenderID() != config.OwnerID {
		return nil
	}

	flag := strings.ToLower(args[1])
	var enabled bool
	switch flag {
	case "on", "enable", "enabled":
		enabled = true
	case "off", "disable", "disabled":
		enabled = false
	default:
		_, err := m.ReplyText(c,
			"<blockquote>𝘜𝘴𝘢𝘨𝘦:\n<code>/welcome on</code>\n<code>/welcome off</code></blockquote>",
			&td.SendTextMessageOpts{ParseMode: td.ParseModeHTML})
		return err
	}

	if err := database.SetWelcomeEnabled(m.ChatID(), enabled); err != nil {
		return err
	}
	if enabled {
		_, err := m.ReplyText(c, "✅ 𝘞𝘦𝘭𝘤𝘰𝘮𝘦 𝘮𝘦𝘴𝘴𝘢𝘨𝘦𝘴 𝘦𝘯𝘢𝘣𝘭𝘦𝘥.", nil)
		return err
	}
	_, err := m.ReplyText(c, "✅ 𝘞𝘦𝘭𝘤𝘰𝘮𝘦 𝘮𝘦𝘴𝘴𝘢𝘨𝘦𝘴 𝘥𝘪𝘴𝘢𝘣𝘭𝘦𝘥.", nil)
	return err
}

func registerWelcomeHandlers(bot *td.Client) {
	// Keep the member-update handler for normal membership changes, but also
	// listen for Telegram's service messages. Depending on how the member
	// joined (invite link, join request approval, or direct add), TDLib can
	// deliver the event as a service message instead of UpdateChatMember.
	bot.OnUpdateChatMember(welcomeChatMemberHandler, nil)
	bot.OnUpdateNewChatJoinRequest(joinRequestHandler, nil)
	bot.OnMessage(welcomeServiceMessageHandler, welcomeServiceMessageFilter)
}

func welcomeServiceMessageFilter(m *td.Message) bool {
	if m == nil || m.ChatID() == 0 {
		return false
	}

	switch m.Content.(type) {
	case *td.MessageChatAddMembers,
		*td.MessageChatJoinByLink,
		*td.MessageChatJoinByRequest:
		return true
	default:
		return false
	}
}

func welcomeServiceMessageHandler(c *td.Client, m *td.Message) error {
	if m == nil || m.ChatID() == 0 {
		return nil
	}

	// Don't run in channels/private chats.
	chat, err := c.GetChat(m.ChatID())
	if err != nil || chat == nil {
		return nil
	}
	if t, ok := chat.Type.(*td.ChatTypeSupergroup); ok && t.IsChannel {
		return nil
	}
	if _, ok := chat.Type.(*td.ChatTypePrivate); ok {
		return nil
	}
	if _, ok := chat.Type.(*td.ChatTypeSecret); ok {
		return nil
	}

	enabled, err := database.IsWelcomeEnabled(m.ChatID())
	if err != nil || !enabled {
		return nil
	}

	userIDs := make([]int64, 0, 1)
	switch content := m.Content.(type) {
	case *td.MessageChatAddMembers:
		userIDs = append(userIDs, content.MemberUserIds...)
	case *td.MessageChatJoinByLink, *td.MessageChatJoinByRequest:
		if id := m.SenderID(); id != 0 {
			userIDs = append(userIDs, id)
		}
	}

	for _, userID := range userIDs {
		if userID == 0 || (c.Me != nil && userID == c.Me.Id) {
			continue
		}

		user, err := c.GetUser(userID)
		if err != nil || user == nil {
			continue
		}

		caption := buildWelcomeCaption(chat.Title, user)
		if _, err := c.SendVideo(m.ChatID(), td.InputFileRemote{Id: config.WelcomeVideoURL}, &td.SendVideoOpts{
			Caption:           caption,
			ParseMode:         td.ParseModeHTML,
			SupportsStreaming: true,
			ReplyMarkup:       core.GetWelcomeMarkup(m.ChatID()),
		}); err == nil {
			continue
		}

		// Video may be unavailable; still send the welcome text/buttons.
		if _, err := c.SendTextMessage(m.ChatID(), caption, &td.SendTextMessageOpts{
			ParseMode:   td.ParseModeHTML,
			ReplyMarkup: core.GetWelcomeMarkup(m.ChatID()),
		}); err != nil {
			return err
		}
	}

	return nil
}

func welcomeChatMemberHandler(c *td.Client, u *td.UpdateChatMember) error {
	if u == nil || u.ChatId == 0 {
		return nil
	}

	oldStatus := memberStatus(u.OldChatMember)
	newStatus := memberStatus(u.NewChatMember)
	if newStatus != "member" && newStatus != "administrator" && newStatus != "creator" {
		return nil
	}
	if oldStatus != "left" && oldStatus != "kicked" {
		return nil
	}

	userID := memberID(u.NewChatMember)
	if userID == 0 {
		return nil
	}
	if c.Me != nil && userID == c.Me.Id {
		return sendBotAddedMessage(c, u.ChatId, u.ActorUserId)
	}

	enabled, err := database.IsWelcomeEnabled(u.ChatId)
	if err != nil || !enabled {
		return nil
	}

	user, err := c.GetUser(userID)
	if err != nil || user == nil {
		return nil
	}
	chat, err := c.GetChat(u.ChatId)
	if err != nil || chat == nil {
		return nil
	}

	caption := buildWelcomeCaption(chat.Title, user)
	_, err = c.SendVideo(u.ChatId, td.InputFileRemote{Id: config.WelcomeVideoURL}, &td.SendVideoOpts{
		Caption:           caption,
		ParseMode:         td.ParseModeHTML,
		SupportsStreaming: true,
		ReplyMarkup:       core.GetWelcomeMarkup(u.ChatId),
	})
	if err == nil {
		return nil
	}

	// If the remote video is unavailable, still keep the welcome feature alive
	// with the requested text and buttons.
	_, textErr := c.SendTextMessage(u.ChatId, caption, &td.SendTextMessageOpts{
		ParseMode:   td.ParseModeHTML,
		ReplyMarkup: core.GetWelcomeMarkup(u.ChatId),
	})
	return textErr
}

func buildWelcomeCaption(chatTitle string, user *td.User) string {
	username := "No Username"
	if user.Usernames != nil && len(user.Usernames.ActiveUsernames) > 0 {
		username = "@" + user.Usernames.ActiveUsernames[0]
	}

	return strings.NewReplacer(
		"{chatname}", htmlEscape(chatTitle),
		"{fullname}", mentionOf(user, user.Id),
		"{username}", htmlEscape(username),
		"{id}", fmt.Sprintf("%d", user.Id),
	).Replace(welcomeCaption)
}

func sendBotAddedMessage(c *td.Client, chatID, addedBy int64) error {

	botMention := "the bot"
	if c.Me != nil {
		botMention = mentionOf(c.Me, c.Me.Id)
	}
	userMention := "you"
	if addedBy != 0 {
		if user, err := c.GetUser(addedBy); err == nil && user != nil {
			userMention = mentionOf(user, addedBy)
		}
	}

	group := "this group"
	if chat, err := c.GetChat(chatID); err == nil && chat != nil && chat.Title != "" {
		group = htmlEscape(chat.Title)
	}

	caption := strings.NewReplacer(
		"{bot}", botMention,
		"{brand}", config.BotBrand,
		"{user}", userMention,
		"{group}", group,
	).Replace(addedCaption)

	_, err := c.SendVideo(chatID, td.InputFileRemote{Id: config.AddMeVideoURL}, &td.SendVideoOpts{
		Caption:           caption,
		ParseMode:         td.ParseModeHTML,
		SupportsStreaming: true,
		ReplyMarkup:       core.GetWelcomeMarkup(chatID),
	})
	if err == nil {
		return nil
	}

	_, err = c.SendTextMessage(chatID, caption, &td.SendTextMessageOpts{
		ParseMode:   td.ParseModeHTML,
		ReplyMarkup: core.GetWelcomeMarkup(chatID),
	})
	return err
}

func joinRequestHandler(c *td.Client, u *td.UpdateNewChatJoinRequest) error {
	if u == nil || u.Request == nil || u.UserChatId == 0 {
		return nil
	}

	user, err := c.GetUser(u.Request.UserId)
	if err != nil || user == nil {
		return nil
	}

	// Telegram supplies UserChatId specifically for the private chat with the
	// requester, so the bot can proactively send onboarding information.
	caption := startCaption(c, u.UserChatId, user, user.Id)
	if video := config.StartVideo(); video != "" {
		if _, err := c.SendVideo(u.UserChatId, td.InputFileRemote{Id: video}, &td.SendVideoOpts{
			Caption:           caption,
			ParseMode:         td.ParseModeHTML,
			SupportsStreaming: true,
			ReplyMarkup:       core.GetStartMarkup(u.UserChatId),
		}); err == nil {
			return nil
		}
	}

	_, err = c.SendTextMessage(u.UserChatId, caption, &td.SendTextMessageOpts{
		ParseMode:   td.ParseModeHTML,
		ReplyMarkup: core.GetStartMarkup(u.UserChatId),
	})
	return err
}

func htmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	).Replace(s)
}
