package codecontrib

import "strings"

// FileClass buckets one changed file into a work type. The defaults exclude
// generated code, vendored trees and build output from person line counts and
// keep lockfiles separate (单列) — tests, docs and configuration are work
// types of their own and are never dropped silently.
type FileClass string

const (
	FileClassSource    FileClass = "source"
	FileClassTest      FileClass = "test"
	FileClassDoc       FileClass = "doc"
	FileClassConfig    FileClass = "config"
	FileClassGenerated FileClass = "generated"
	FileClassVendor    FileClass = "vendor"
	FileClassLockfile  FileClass = "lockfile"
	FileClassBinary    FileClass = "binary"
)

// FileChange is one changed file inside a commit or request diff, with its
// work-type classification resolved at ingest time. Binary is reported by the
// provider API and must be stored as FileClassBinary regardless of the path
// (binary blobs carry no comparable line counts).
type FileChange struct {
	Path      string    `json:"path"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
	Binary    bool      `json:"binary"`
	Class     FileClass `json:"class"`
}

// FilterRules extends the built-in classification defaults per workspace or
// per repository. Patterns are case-insensitive: a `*`-prefixed pattern
// (`*.pb.go`, `*_test.go`) matches by suffix, anything else matches as a
// substring (so `vendor/` covers every vendored directory). CountAsSource
// wins over every other rule, which keeps the configuration flexible while
// letting a misconfigured filter only ever hide activity, never invent it.
//
// Binary is not classifiable from the path: collectors set FileClassBinary
// directly when the provider API reports a binary blob.
type FilterRules struct {
	ExtraLockfilePatterns  []string `json:"extra_lockfile_patterns,omitempty"`
	ExtraGeneratedPatterns []string `json:"extra_generated_patterns,omitempty"`
	ExtraVendorPatterns    []string `json:"extra_vendor_patterns,omitempty"`
	CountAsSource          []string `json:"count_as_source,omitempty"`
}

var defaultLockfilePatterns = []string{
	"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lockb",
	"Cargo.lock", "poetry.lock", "uv.lock", "composer.lock",
	"Gemfile.lock", "Podfile.lock", "packages.lock.json", "go.sum",
}

var defaultGeneratedPatterns = []string{
	"*.pb.go", "*.pb.gw.go", "*.gen.go", "*.g.dart", "*_pb2.py",
	"*.min.js", "*.min.css", "*.generated.*", "__generated__/",
}

var defaultVendorPatterns = []string{
	"vendor/", "node_modules/", "third_party/",
	"dist/", "build/", "out/",
}

var defaultTestPatterns = []string{
	"*_test.go", "*.test.ts", "*.test.tsx", "*.test.js",
	"*.spec.ts", "*.spec.tsx", "*.spec.js",
	"test/", "tests/", "__tests__/",
}

var defaultDocPatterns = []string{"*.md", "*.mdx", "*.rst", "docs/"}

var defaultConfigPatterns = []string{
	"*.json", "*.yaml", "*.yml", "*.toml", "*.ini", "*.conf", "*.env",
	"dockerfile", "makefile", "go.mod",
}

// ClassifyPath maps one changed file path to its work type. Lookup order is
// fixed and documented: override → lockfile → generated → vendor → test →
// doc → config → source, so the same path always classifies the same way
// under the same rules version.
func ClassifyPath(rules FilterRules, path string) FileChange {
	class := classifyPath(rules, path)
	return FileChange{Path: path, Class: class}
}

func classifyPath(rules FilterRules, path string) FileClass {
	if matchAny(rules.CountAsSource, path) {
		return FileClassSource
	}
	if matchAny(defaultLockfilePatterns, path) || matchAny(rules.ExtraLockfilePatterns, path) {
		return FileClassLockfile
	}
	if matchAny(defaultGeneratedPatterns, path) || matchAny(rules.ExtraGeneratedPatterns, path) {
		return FileClassGenerated
	}
	if matchAny(defaultVendorPatterns, path) || matchAny(rules.ExtraVendorPatterns, path) {
		return FileClassVendor
	}
	switch {
	case matchAny(defaultTestPatterns, path):
		return FileClassTest
	case matchAny(defaultDocPatterns, path):
		return FileClassDoc
	case matchAny(defaultConfigPatterns, path):
		return FileClassConfig
	default:
		return FileClassSource
	}
}

func matchAny(patterns []string, path string) bool {
	lower := strings.ToLower(path)
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if strings.HasPrefix(pattern, "*") {
			if strings.HasSuffix(lower, pattern[1:]) {
				return true
			}
			continue
		}
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
