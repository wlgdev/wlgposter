package app

import (
	"fmt"
)

func (a *App) onTelegramError(err error) {
	if err == nil {
		return
	}

	a.log.Error().Err(err).Msg("Telegram post processing failed")
	a.tg.MessageToAdmins(fmt.Sprintf("❌ Ошибка Telegram: `%s`", err.Error()))
}
