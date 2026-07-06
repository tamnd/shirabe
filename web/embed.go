// Package web embeds the UI assets served by pkg/server.
package web

import "embed"

//go:embed index.html style.css app.js cards.js
var FS embed.FS
