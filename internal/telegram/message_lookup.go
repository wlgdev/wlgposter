package telegram

import (
	gogram "github.com/amarnathcjd/gogram/telegram"
)

func (t *Telegram) GetChannelMessages(channelID, accessHash int64, ids []int32) ([]gogram.NewMessage, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	return t.Client.GetMessages(&gogram.InputPeerChannel{
		ChannelID:  channelID,
		AccessHash: accessHash,
	}, &gogram.SearchOption{
		IDs: ids,
	})
}
