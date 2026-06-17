package connector

// hasOddDelimRun reports whether s contains a run of the delimiter character
// with odd length. Used to verify that quoted identifiers have properly escaped
// delimiter characters (all even-length runs = properly paired).
func hasOddDelimRun(s string, delim byte) bool {
	count := 0
	for i := range len(s) {
		if s[i] == delim {
			count++
		} else {
			if count%2 != 0 {
				return true
			}
			count = 0
		}
	}
	return count%2 != 0
}
