package paldata

// Relic types are enum members in the save — EPalRelicType::CapturePower and
// so on. The Korean labels here are ours; the game ships no table for them
// that we have.
//
// The list is what a live save actually held across seven players. Different
// players carried different subsets — counts of 2, 9, 11 and 12 entries all
// appeared — so this is a union, not a fixed roster, and an unknown type is
// shown under its raw key rather than dropped.

const relicPrefix = "EPalRelicType::"

// Relic pairs the save's enum member with a Korean label.
type Relic struct {
	ID     string `json:"id"`
	NameKO string `json:"nameKo"`
}

// relicOrder puts capture power first, since the Statue of Power is the one
// most people come here for.
var relicOrder = []Relic{
	{"CapturePower", "포획률 (힘의 석상)"},
	{"MoveSpeed", "이동 속도"},
	{"ClimbSpeed", "등반 속도"},
	{"SwimSpeed", "수영 속도"},
	{"GliderSpeed", "글라이더 속도"},
	{"JumpPower", "점프력"},
	{"StaminaReduction", "스태미나 소모 감소"},
	{"HungerReduction", "허기 감소"},
	{"FoodDecayReduction", "음식 부패 감소"},
	{"StatusAilmentResist", "상태이상 저항"},
	{"SphereHoming", "팰 스피어 유도"},
	{"ExpBonus", "경험치 보너스"},
	{"RainbowPassiveRate", "무지개 패시브 확률"},
}

// Relics lists every known relic type, capture power first.
func Relics() []Relic {
	out := make([]Relic, len(relicOrder))
	copy(out, relicOrder)
	return out
}

// RelicName returns the Korean label for a relic type, accepting either the
// bare name or the full enum member.
func RelicName(id string) (string, bool) {
	bare := TrimRelicPrefix(id)
	for _, r := range relicOrder {
		if r.ID == bare {
			return r.NameKO, true
		}
	}
	return "", false
}

// TrimRelicPrefix reduces an enum member to its bare name.
func TrimRelicPrefix(id string) string {
	if len(id) > len(relicPrefix) && id[:len(relicPrefix)] == relicPrefix {
		return id[len(relicPrefix):]
	}
	return id
}

// QualifyRelic is the inverse: the form the save stores.
func QualifyRelic(bare string) string {
	if len(bare) > len(relicPrefix) && bare[:len(relicPrefix)] == relicPrefix {
		return bare
	}
	return relicPrefix + bare
}
