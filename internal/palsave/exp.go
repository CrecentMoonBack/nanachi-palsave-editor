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
