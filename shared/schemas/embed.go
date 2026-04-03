package schemas

import "embed"

//go:embed *.json defs/*.json
var FS embed.FS
