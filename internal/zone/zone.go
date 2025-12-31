package zone

import "log"

// RestrictedZones is the opinionated list of zones where casting is not allowed (e.g., towns).
var RestrictedZones = map[string]bool{
	// San d'Oria
	"Zone_230": true, // Southern San d'Oria
	"Zone_231": true, // Northern San d'Oria
	"Zone_232": true, // Port San d'Oria

	// Bastok
	"Zone_234": true, // Port Bastok
	"Zone_235": true, // Bastok Mines
	"Zone_236": true, // Bastok Markets
	"Zone_237": true, // Metalworks (part of Bastok, often considered restricted)

	// Windurst
	"Zone_238": true, // Port Windurst
	"Zone_239": true, // Windurst Waters
	"Zone_240": true, // Windurst Walls
	"Zone_241": true, // Windurst Woods
	"Zone_242": true, // Heaven's Tower

	// Jeuno
	"Zone_243": true, // Ru'Lude Gardens
	"Zone_244": true, // Upper Jeuno
	"Zone_245": true, // Lower Jeuno
	"Zone_246": true, // Port Jeuno

	// Aht Urhgan
	"Zone_050": true, // Al Zahbi
	"Zone_053": true, // Nashmau
	"Zone_087": true, // The Colosseum (event area, often restricted)
	"Zone_258": true, // Aht Urhgan Whitegate (likely ID, common hub town)

	// Other major towns/ports
	"Zone_247": true, // Kazham
	"Zone_248": true, // Mhaura
	"Zone_249": true, // Norg
	"Zone_252": true, // Rabao
	"Zone_280": true, // Tavnazian Safehold
	"Zone_284": true, // Southern San d'Oria [S] (if including past versions)
	"Zone_233": true, // Selbina
}

// IsRestricted checks if a zone ID is in the restricted list.
func IsRestricted(zoneID string) bool {
	if RestrictedZones[zoneID] {
		log.Printf("[ZONE] Casting restricted in %s", zoneID)
		return true
	}
	return false
}
