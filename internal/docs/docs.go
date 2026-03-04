// Package docs provides embedded documentation for envctl CLI topics.
package docs

import _ "embed"

//go:embed content/overview.md
var Overview string

//go:embed content/config.md
var Config string

//go:embed content/examples.md
var Examples string

//go:embed content/k8s.md
var K8s string

//go:embed content/patterns.md
var Patterns string

//go:embed content/onepassword.md
var OnePassword string
