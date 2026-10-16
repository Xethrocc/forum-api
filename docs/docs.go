// Package docs stellt die API-Dokumentation als eingebettete Dateien bereit.
package docs

import _ "embed"

// OpenAPIYAML ist die maschinenlesbare API-Beschreibung,
// ausgeliefert unter /openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPIYAML []byte

// RedocJS ist das lokal ausgelieferte Redoc-Bundle für /docs.
// Bewusst kein CDN: die Seite muss auch mit Werbeblockern funktionieren.
//
//go:embed redoc.standalone.js
var RedocJS []byte
