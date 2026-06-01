package maintenance

import (
	"regexp"
	"strings"
)

type Target string

const (
	TargetFrontend Target = "frontend"
	TargetBackend  Target = "backend"
	TargetInfra    Target = "infra"
)

var (
	feSignals = []string{
		"page", "button", "link", "navbar", "nav", "menu", "header", "footer",
		"hero", "image", "photo", "gallery", "css", "style", "styling", "color",
		"colour", "font", "layout", "mobile", "responsive", "screen", "click",
		"scroll", "modal", "dropdown", "form", "ui", "ux", "display", "text",
		"typo", "spelling", "wording", "donate button", "story", "stories",
		"blog", "card", "icon", "logo", "frontend", "react", "spelling",
	}
	beSignals = []string{
		"api", "endpoint", "database", "db", "postgres", "sql", "migration",
		"server", "backend", "auth", "login", "jwt", "token", "rate limit",
		"webhook", "stripe", "payment", "donation", "email", "contact form",
		"translate", "translation", "cache", "redis", "500", "502", "503",
		"timeout", "crash", "panic", "go ", "golang", "handler", "service",
	}
	infraSignals = []string{
		"azure", "terraform", "infra", "infrastructure", "deploy", "deployment",
		"dns", "domain", "ssl", "certificate", "cert", "key vault", "keyvault",
		"secret", "container", "registry", "app service", "static web app",
		"cdn", "blob storage", "scaling", "cost", "billing", "pipeline",
		"github actions", "workflow", "ci", "cd",
	}
)

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

func Classify(title, description string) Target {
	text := strings.ToLower(title + " " + description)

	score := func(signals []string) int {
		n := 0
		for _, s := range signals {
			if strings.Contains(text, s) {
				n++
			}
		}
		return n
	}

	fe := score(feSignals)
	be := score(beSignals)
	infra := score(infraSignals)

	best := TargetFrontend
	bestScore := fe
	if be > bestScore {
		best, bestScore = TargetBackend, be
	}
	if infra > bestScore {
		best, bestScore = TargetInfra, infra
	}
	return best
}
