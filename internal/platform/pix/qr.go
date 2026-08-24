package pix

import (
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// QRSVG renders the BR Code as a scalable QR symbol. SVG (not PNG) so the
// same string prints crisply on a phone, a desktop and a sheet of paper —
// and so the site can paint it in the couple's own ink.
//
// Medium recovery is the level BACEN's manual recommends for BR Codes: it
// survives a scratched print without inflating the symbol.
func QRSVG(brCode string) (string, error) {
	code, err := qrcode.New(brCode, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("build qr: %w", err)
	}
	// Bitmap() already frames the symbol with the mandatory quiet zone.
	modules := code.Bitmap()
	size := len(modules)

	// One path for every dark module keeps the markup small and lets the
	// caller colour the whole symbol with `fill`.
	var path strings.Builder
	for y, row := range modules {
		for x, dark := range row {
			if dark {
				fmt.Fprintf(&path, "M%d %dh1v1h-1z", x, y)
			}
		}
	}

	var svg strings.Builder
	fmt.Fprintf(&svg,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges" role="img">`,
		size, size)
	svg.WriteString(`<path fill="currentColor" d="`)
	svg.WriteString(path.String())
	svg.WriteString(`"/></svg>`)
	return svg.String(), nil
}
