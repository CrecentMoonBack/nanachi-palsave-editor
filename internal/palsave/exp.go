package palsave

// palTotalExp is the cumulative experience a pal needs to be at each level,
// indexed by level-1. Extracted from the game's own level curve.
//
// This lives in palsave rather than paldata because it is needed for
// correctness, not display: a pal whose Level and Exp disagree gets one
// re-derived from the other by the game, silently undoing an edit.
var palTotalExp = [...]int64{
	0, 25, 56, 93, 138, 207,
	306, 440, 616, 843, 1131, 1492,
	1941, 2495, 3175, 4007, 5021, 6253,
	7747, 9555, 11740, 14378, 17559, 21392,
	26007, 31561, 38241, 46272, 55925, 67524,
	81458, 98195, 118294, 142429, 171406, 206194,
	247955, 298084, 358255, 430475, 517155, 621186,
	746039, 895878, 1075701, 1291504, 1550483, 1861273,
	2234236, 2681807, 3218908, 3863445, 4636905, 5565072,
	6678888, 8015483, 9619412, 11544143, 13853835, 16625481,
	19951472, 23942677, 28732138, 34479507, 41376365, 48273223,
	55170081, 62066939, 68963797, 75860655, 82757513, 89654371,
	96551229, 103448087, 110344945, 117241803, 124138661, 131035519,
	137932377, 144829235, 151726093, 158622951, 165519809, 172416667,
	179313525, 186210383, 193107241, 200004099, 206900957, 213797815,
	220694673, 227591531, 234488389, 241385247, 248282105, 255178963,
	262075821, 268972679, 275869537, 282766395,
}

// MaxKnownLevel is the highest level the experience curve defines.
//
// The game's own cap is lower — 65 at time of writing — but saves in the wild
// contain higher, and the curve goes to 100, so the editor allows it rather
// than second-guessing.
const MaxKnownLevel = len(palTotalExp)

// TotalPalExpForLevel returns the cumulative experience matching a level.
func TotalPalExpForLevel(level int) (int64, bool) {
	if level < 1 || level > MaxKnownLevel {
		return 0, false
	}
	return palTotalExp[level-1], true
}

// LevelForTotalPalExp is the inverse: the level an experience total implies.
func LevelForTotalPalExp(exp int64) int {
	level := 1
	for i, need := range palTotalExp {
		if exp < need {
			break
		}
		level = i + 1
	}
	return level
}

// playerTotalExp is the cumulative experience a *player* needs at each level,
// indexed by level-1. Players and pals are on entirely different curves, and
// using the pal one for a player loses data rather than approximating: at
// level 80 a pal needs 144,829,235 and a player 45,859,908, so writing the pal
// figure onto a player leaves them holding three times the experience their
// level calls for. The game re-derives level from experience the moment any is
// gained, so the edited level springs straight back — exactly what a user
// reported after lowering their level and eating one egg.
//
// Verified against the live save: all eight players sit inside
// [playerTotalExp[level-1], playerTotalExp[level]), which is what being at
// that level means.
var playerTotalExp = [...]int64{
	0, 50, 200, 400, 740, 1260,
	1960, 2860, 4010, 5510, 7410, 9810,
	12810, 16610, 21287, 26640, 32500, 38867,
	47431, 56840, 68784, 81573, 97404, 115094,
	136164, 158924, 185064, 212894, 242414, 273624,
	308170, 346399, 388694, 435477, 487215, 544422,
	607667, 677577, 754844, 840233, 934588, 1038842,
	1154024, 1281269, 1421830, 1577090, 1748577, 1937977,
	2147151, 2378134, 2633193, 2914828, 3225800, 3569156,
	3947260, 4364833, 4824976, 5331215, 5887550, 6498533,
	7169350, 7905913, 8714987, 9604263, 10582213, 11658031,
	12845734, 14156947, 15604507, 17202587, 18966831, 20914510,
	23064692, 25438426, 28058950, 30951910, 34145608, 37671270,
	41563358, 45859908, 47456958, 49389388, 51727629, 54556901,
	57980320, 62122657, 67134884, 73199679, 80538080, 89417545,
	100161699, 113162125, 128892641, 147926565, 170957613, 198825181,
	232544939, 273345845, 322714941, 382451548,
}

// MaxKnownPlayerLevel is the highest level the player curve defines.
const MaxKnownPlayerLevel = len(playerTotalExp)

// TotalPlayerExpForLevel returns the cumulative experience matching a player
// level — the floor of that level's band, so the level reads back correctly
// and no progress toward the next one is invented.
func TotalPlayerExpForLevel(level int) (int64, bool) {
	if level < 1 || level > MaxKnownPlayerLevel {
		return 0, false
	}
	return playerTotalExp[level-1], true
}

// LevelForTotalPlayerExp is the inverse: the level a player's experience
// implies, which is what the game itself computes on the next exp gain.
func LevelForTotalPlayerExp(exp int64) int {
	level := 1
	for i, need := range playerTotalExp {
		if exp < need {
			break
		}
		level = i + 1
	}
	return level
}
