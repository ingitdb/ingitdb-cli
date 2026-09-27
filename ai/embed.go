// Package ai embeds ai/skills -- inGitDB's canonical, harness-neutral Agent
// Skills -- directly into the ingitdb binary.
package ai

import "embed"

// SkillsFS holds every file under skills/ at build time: skills/<name>/
// SKILL.md plus each skill's references/ subdirectories.
//
//go:embed all:skills
var SkillsFS embed.FS
