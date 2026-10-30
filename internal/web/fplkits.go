package web

import (
	"fmt"
	"html/template"
	"sort"
	"strings"
)

// fplKit is a club's home shirt, drawn as a small SVG beside each player in
// the matchup panel instead of the club's three-letter code.
type fplKit struct {
	Body    string
	Stripe  string // second stripe colour, "" for a plain shirt
	Sleeves string // "" to match the body
	Trim    string // collar
}

// fplKits is keyed by FPL's short club name. Unknown clubs fall back to
// the code as text.
var fplKits = map[string]fplKit{
	"ARS": {Body: "#EF0107", Sleeves: "#FFFFFF", Trim: "#FFFFFF"},
	"AVL": {Body: "#670E36", Sleeves: "#95BFE5", Trim: "#95BFE5"},
	"BOU": {Body: "#DA291C", Stripe: "#111111", Trim: "#111111"},
	"BRE": {Body: "#E30613", Stripe: "#FFFFFF", Trim: "#111111"},
	"BHA": {Body: "#0057B8", Stripe: "#FFFFFF", Trim: "#0057B8"},
	"CHE": {Body: "#034694", Trim: "#FFFFFF"},
	"COV": {Body: "#59CBE8", Trim: "#FFFFFF"},
	"CRY": {Body: "#1B458F", Stripe: "#C4122E", Trim: "#1B458F"},
	"EVE": {Body: "#003399", Trim: "#FFFFFF"},
	"FUL": {Body: "#FFFFFF", Trim: "#111111"},
	"HUL": {Body: "#F5A12D", Stripe: "#111111", Trim: "#111111"},
	"IPS": {Body: "#0044A9", Trim: "#FFFFFF"},
	"LEE": {Body: "#FFFFFF", Trim: "#1D428A"},
	"LIV": {Body: "#C8102E", Trim: "#F6EB61"},
	"MCI": {Body: "#6CABDD", Trim: "#FFFFFF"},
	"MUN": {Body: "#DA291C", Trim: "#111111"},
	"NEW": {Body: "#FFFFFF", Stripe: "#111111", Trim: "#111111"},
	"NFO": {Body: "#DD0000", Trim: "#FFFFFF"},
	"TOT": {Body: "#FFFFFF", Trim: "#132257", Sleeves: "#FFFFFF"},
	"SUN": {Body: "#EB172B", Stripe: "#FFFFFF", Trim: "#111111"},
}

const (
	kitBody        = "M8 3 5 4 2 8.2 4.6 10.4 6 9.4V21h12V9.4l1.4 1 2.6-2.2L19 4l-3-1c-.9 1.3-2.3 2-4 2s-3.1-.7-4-2z"
	kitLeftSleeve  = "M5 4 2 8.2 4.6 10.4 6.4 9.1 7.2 4.6z"
	kitRightSleeve = "M19 4l3 4.2-2.6 2.2-1.8-1.3-.8-4.5z"
	kitCollar      = "M8 3c.9 1.3 2.3 2 4 2s3.1-.7 4-2"
)

// fplKitSprite is every club's shirt as an SVG symbol, included once per
// page and referenced by fplKitIcon.
func fplKitSprite() template.HTML {
	clubs := make([]string, 0, len(fplKits))
	for c := range fplKits {
		clubs = append(clubs, c)
	}
	sort.Strings(clubs)

	var b strings.Builder
	b.WriteString(`<svg width="0" height="0" style="position:absolute" aria-hidden="true"><defs>`)
	for _, c := range clubs {
		k := fplKits[c]
		if k.Stripe != "" {
			fmt.Fprintf(&b, `<pattern id="kit-%s-stripes" patternUnits="userSpaceOnUse" width="4" height="24"><rect width="2" height="24" fill="%s"/><rect x="2" width="2" height="24" fill="%s"/></pattern>`, c, k.Body, k.Stripe)
		}
	}
	b.WriteString(`</defs>`)
	for _, c := range clubs {
		k := fplKits[c]
		fill := k.Body
		if k.Stripe != "" {
			fill = "url(#kit-" + c + "-stripes)"
		}
		fmt.Fprintf(&b, `<symbol id="kit-%s" viewBox="0 0 24 24"><path d="%s" fill="%s"/>`, c, kitBody, fill)
		if k.Sleeves != "" {
			fmt.Fprintf(&b, `<path d="%s" fill="%s"/><path d="%s" fill="%s"/>`, kitLeftSleeve, k.Sleeves, kitRightSleeve, k.Sleeves)
		}
		fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="1.2"/><path d="%s" fill="none" stroke="rgba(0,0,0,.28)" stroke-width=".6" stroke-linejoin="round"/></symbol>`, kitCollar, k.Trim, kitBody)
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// fplKitIcon is a club's shirt, or its code as text if the club is unknown.
func fplKitIcon(club string) template.HTML {
	esc := template.HTMLEscapeString(club)
	if _, ok := fplKits[club]; !ok {
		return template.HTML(`<span class="mc-club">` + esc + `</span>`)
	}
	return template.HTML(`<svg class="kit" role="img" aria-label="` + esc + `"><title>` + esc + `</title><use href="#kit-` + esc + `"/></svg>`)
}
