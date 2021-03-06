package decorators

import (
	"github.com/TF2Stadium/Helen/models"
	"github.com/bitly/go-simplejson"
)

func GetPlayerSettingsJson(settings []models.PlayerSetting) *simplejson.Json {
	json := simplejson.New()

	for _, obj := range settings {
		json.Set(obj.Key, obj.Value)
	}

	return json
}

func GetPlayerProfileJson(p *models.Player) *simplejson.Json {
	j := simplejson.New()

	// stats
	s := simplejson.New()
	s.Set("playedHighlanderCount", p.Stats.PlayedHighlanderCount)
	s.Set("playedSixesCount", p.Stats.PlayedSixesCount)

	// info
	j.Set("createdAt", p.CreatedAt)
	j.Set("gameHours", p.GameHours)
	j.Set("steamid", p.SteamId)
	j.Set("avatar", p.Avatar)
	j.Set("stats", s)
	j.Set("name", p.Name)
	j.Set("id", p.ID)

	return j
}
