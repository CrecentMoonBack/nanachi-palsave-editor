package paldata

// Player status point names are Japanese in the save — 最大HP, 攻撃力 and so
// on. They are internal keys rather than display text, and the game ships no
// Korean table for them that we have, so the labels here are ours.
//
// Unknown names are shown as-is rather than hidden: a game update adding a
// status should surface in the editor, not vanish from it.

// StatusPoint pairs the save's own key with a Korean label.
type StatusPoint struct {
	ID     string `json:"id"`
	NameKO string `json:"nameKo"`
}

// statusPointOrder is the order the game lists them in.
var statusPointOrder = []StatusPoint{
	{"最大HP", "최대 체력"},
	{"最大SP", "최대 스태미나"},
	{"攻撃力", "공격력"},
	{"所持重量", "소지 중량"},
	{"捕獲率", "포획률"},
	{"作業速度", "작업 속도"},
	{"食料腐敗低減", "음식 부패 감소"},
	{"ジャンプ力", "점프력"},
	{"パルスフィアホーミング", "팰 스피어 유도"},
	{"経験値ボーナス", "경험치 보너스"},
	{"移動速度アップ", "이동 속도"},
}

// StatusPoints lists every known status in the game's order.
func StatusPoints() []StatusPoint {
	out := make([]StatusPoint, len(statusPointOrder))
	copy(out, statusPointOrder)
	return out
}

// StatusPointName returns the Korean label for a save key.
//
// ok is false for a key we have no label for, so a caller can show the raw
// name rather than an empty string.
func StatusPointName(id string) (string, bool) {
	for _, s := range statusPointOrder {
		if s.ID == id {
			return s.NameKO, true
		}
	}
	return "", false
}
