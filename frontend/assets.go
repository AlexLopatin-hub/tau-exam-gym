package frontend

import "embed"

//go:embed static/app.css static/app.js
var Assets embed.FS
