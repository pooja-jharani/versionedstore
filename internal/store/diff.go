package store

import "strings"

// LineDiff produces a simple unified-style line diff between two values
// using an LCS (longest common subsequence) alignment, implemented from
// scratch so the store stays zero-dependency. Lines are prefixed "  "
// (unchanged), "- " (removed), or "+ " (added).
func LineDiff(oldVal, newVal string) string {
	oldLines := strings.Split(oldVal, "\n")
	newLines := strings.Split(newVal, "\n")

	n, m := len(oldLines), len(newLines)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var b strings.Builder
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case oldLines[i] == newLines[j]:
			b.WriteString("  " + oldLines[i] + "\n")
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			b.WriteString("- " + oldLines[i] + "\n")
			i++
		default:
			b.WriteString("+ " + newLines[j] + "\n")
			j++
		}
	}
	for ; i < n; i++ {
		b.WriteString("- " + oldLines[i] + "\n")
	}
	for ; j < m; j++ {
		b.WriteString("+ " + newLines[j] + "\n")
	}
	return b.String()
}
