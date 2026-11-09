package app

import (
	"regexp"
	"strings"
	"time"

	"blonymonitorv2/internal/packet"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SmugglerNotice contains only a locally captured game announcement.
type SmugglerNotice struct {
	Type       string `json:"type"`
	Area       string `json:"area"`
	Item       string `json:"item"`
	ReceivedAt int64  `json:"receivedAt"`
}

var (
	smugglerForecast = regexp.MustCompile(`^在\s*4\s*分钟后地区的(.+?)处[，,]\s*出现了走私(?:贩子|商人)[。.．]?\s*今天的贸易物品为(.+?)[。.．]?$`)
	smugglerAppear   = regexp.MustCompile(`^在(.+?)地区出现了走私(?:贩子|商人)[。.．]\s*今天的贸易物品为(.+?)[。.．]?$`)
	smugglerEnding   = regexp.MustCompile(`^(.+?)地区的走私(?:贩子|商人)\s*4\s*分钟后将会消失[。.．]?$`)
)

func parseSmugglerNotice(text string) *SmugglerNotice {
	text = strings.TrimSpace(strings.TrimRight(text, "\x00"))
	for _, pattern := range []struct {
		kind string
		re   *regexp.Regexp
	}{
		{"forecast", smugglerForecast},
		{"appear", smugglerAppear},
		{"disappear_forecast", smugglerEnding},
	} {
		match := pattern.re.FindStringSubmatch(text)
		if match == nil {
			continue
		}
		notice := &SmugglerNotice{Type: pattern.kind, Area: strings.TrimSpace(match[1])}
		if len(match) > 2 {
			notice.Item = strings.TrimSpace(match[2])
		}
		return notice
	}
	return nil
}

// Announcement phases are 19:00, 22:00 and 03:00 (on the following Eirin day).
func smugglerPhaseOffset(kind string) time.Duration {
	switch kind {
	case "appear":
		return 270 * time.Second
	case "disappear_forecast":
		return 720 * time.Second
	default:
		return 0
	}
}

func (a *App) handleSmugglerNotice(pkt *packet.GamePacket) bool {
	if pkt == nil || pkt.Op != opcodeChineseName {
		return false
	}
	for _, elem := range pkt.Msg {
		if elem == nil || elem.Type() != packet.MessageElemTypeString {
			continue
		}
		notice := parseSmugglerNotice(elem.Data().(string))
		if notice == nil {
			continue
		}
		at := pkt.At
		if at.IsZero() {
			at = time.Now()
		}
		notice.ReceivedAt = at.UnixMilli()
		a.mu.Lock()
		previous := a.smugglerNotice
		if previous != nil && previous.ReceivedAt >= notice.ReceivedAt {
			a.mu.Unlock()
			return true
		}
		if previous != nil && notice.Item == "" && strings.Join(strings.Fields(previous.Area), "") == strings.Join(strings.Fields(notice.Area), "") {
			previousAnchor := previous.ReceivedAt - smugglerPhaseOffset(previous.Type).Milliseconds()
			anchor := notice.ReceivedAt - smugglerPhaseOffset(notice.Type).Milliseconds()
			// Recover the item from the same appearance only, never yesterday's.
			if delta := anchor - previousAnchor; delta >= -90_000 && delta <= 90_000 {
				notice.Item = previous.Item
			}
		}
		a.smugglerNotice = notice
		a.mu.Unlock()
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "smuggler-notice", notice)
		}
		return true
	}
	return false
}

func (a *App) GetSmugglerNotice() *SmugglerNotice {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.smugglerNotice == nil {
		return nil
	}
	notice := *a.smugglerNotice
	return &notice
}
