package app

import (
	"fmt"
	"strings"
	"wlgposter/internal/cache"

	gogram "github.com/amarnathcjd/gogram/telegram"
)

func (a *App) onAdminMessage(message *gogram.NewMessage) {
	split := strings.Fields(message.Text())
	if len(split) > 0 {
		switch split[0] {
		case "/ping":
			a.commandPing(message)
			return
		case "/banword":
			a.commandBanword(split, message)
			return
		}
	}
}

func (a *App) commandPing(message *gogram.NewMessage) {
	a.logAdminCommand(message)
	message.Respond("pong", &gogram.SendOptions{ParseMode: "markdown"})
}

func (a *App) commandBanword(split []string, message *gogram.NewMessage) {
	a.logAdminCommand(message)

	if len(split) < 2 {
		message.Respond("🚫 Использование: /banword list | add <слово> | add_vk <слово> | add_max <слово> | delete <слово>", &gogram.SendOptions{ParseMode: "markdown"})
		return
	}
	banwords, err := a.bannedWords.List()
	if err != nil {
		a.log.Error().Err(err).Msg("Banned words list failed")
	}

	arg := strings.Join(split[2:], " ")

	var msg string
	switch split[1] {
	case "list":
		if len(banwords) == 0 {
			msg = "Список пуст"
			break
		}

		var wordsList []string
		for _, bw := range banwords {
			wordsList = append(wordsList, fmt.Sprintf("%s (VK:%t, Max:%t)", bw.Word, bw.PlatformVK, bw.PlatformMax))
		}
		msg = fmt.Sprintf("Бан ворды:\n```\n%s\n```", strings.Join(wordsList, "\n"))

	case "add", "add_vk", "add_max":
		if arg == "" {
			msg = "Нужно указать слово для добавления"
			break
		}

		var word *cache.BannedWord
		for _, bw := range banwords {
			if bw.Word == arg {
				word = &bw
				break
			}
		}
		if word != nil {
			msg = fmt.Sprintf("Бан ворд уже есть: `%s` (VK:%t, Max:%t)", word.Word, word.PlatformVK, word.PlatformMax)
			break
		}

		var platformVk, platformMax bool
		switch split[1] {
		case "add_vk":
			platformVk = true
			platformMax = false
		case "add_max":
			platformVk = false
			platformMax = true
		default:
			platformVk = true
			platformMax = true
		}

		if err := a.bannedWords.Add(arg, platformVk, platformMax); err != nil {
			a.log.Error().Err(err).Msg("Banned words add failed")
			msg = "Не удалось добавить бан ворд"
			break
		}
		msg = fmt.Sprintf("Бан ворд добавлен: `%s` (VK:%t, Max:%t)", arg, platformVk, platformMax)
	case "delete":
		if arg == "" {
			msg = "Нужно указать слово для удаления"
			break
		}
		deleted, err := a.bannedWords.Delete(arg)
		if err != nil {
			a.log.Error().Err(err).Msg("Banned words delete failed")
			msg = "Не удалось удалить бан ворд"
			break
		}
		if !deleted {
			msg = fmt.Sprintf("Бан ворд не найден:`%s`", arg)
			break
		}
		msg = fmt.Sprintf("Бан ворд удален: `%s`", arg)
	}

	message.Respond(fmt.Sprintf("🚫 %s", msg), &gogram.SendOptions{ParseMode: "markdown"})
}

func (a *App) logAdminCommand(message *gogram.NewMessage) {
	a.log.Info().
		Int64("chat_id", message.ChatID()).
		Str("username", message.Sender.Username).
		Str("text", message.Text()).
		Msg("Telegram admin command")
}
