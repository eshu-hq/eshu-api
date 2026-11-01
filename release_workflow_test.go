// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package ciworkflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	releaseWorkflow = ".github/workflows/release.yml"
	semverStep      = "Require a semantic version tag"
	mainHeadStep    = "Require the tag to point at the protected main head"
)

// releaseStep returns the body of the named step in release.yml, from its
// `- name:` line up to the next step, with comments stripped.
func releaseStep(t *testing.T, name string) string {
	t.Helper()

	var (
		body  []string
		found bool
	)
	for _, line := range strings.Split(stripComments(readFile(t, releaseWorkflow)), "\n") {
		if strings.HasPrefix(line, "      - ") {
			if found {
				break
			}
			found = strings.TrimSpace(strings.TrimPrefix(line, "      - ")) == "name: "+name
		}
		if found {
			body = append(body, line)
		}
	}
	if !found {
		t.Fatalf("release.yml has no step named %q", name)
	}
	return strings.Join(body, "\n")
}

// stepRun extracts the `run: |` script of a step body, dedented.
func stepRun(t *testing.T, step string) string {
	t.Helper()

	var (
		script []string
		inRun  bool
	)
	for _, line := range strings.Split(step, "\n") {
		if strings.TrimSpace(line) == "run: |" {
			inRun = true
			continue
		}
		if inRun {
			script = append(script, strings.TrimPrefix(line, "          "))
		}
	}
	if len(script) == 0 {
		t.Fatalf("step has no run script:\n%s", step)
	}
	return strings.Join(script, "\n")
}

// runScript executes a step script with bash in dir, returning its exit code.
func runScript(t *testing.T, dir, script string, env ...string) int {
	t.Helper()

	cmd := exec.Command("bash", "-c", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("running script: %v\n%s", err, out)
	}
	return exitErr.ExitCode()
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestReleaseTagGuardsRunBeforeAnyBuildOrPush pins that both tag guards are
// tag-only steps and sit before every step that builds, logs in, or pushes.
func TestReleaseTagGuardsRunBeforeAnyBuildOrPush(t *testing.T) {
	text := stripComments(readFile(t, releaseWorkflow))

	for _, name := range []string{semverStep, mainHeadStep} {
		step := releaseStep(t, name)
		if !strings.Contains(step, "if: github.ref_type == 'tag'") {
			t.Errorf("step %q must be gated on github.ref_type == 'tag':\n%s", name, step)
		}
		at := strings.Index(text, "- name: "+name)
		for _, later := range []string{
			"docker/setup-buildx-action@",
			"docker/login-action@",
			"docker/metadata-action@",
			"docker/build-push-action@",
		} {
			if i := strings.Index(text, later); i < at {
				t.Errorf("step %q must come before %s (guard at %d, action at %d)", name, later, at, i)
			}
		}
	}
	if !strings.Contains(releaseStep(t, mainHeadStep), "TAG_COMMIT: ${{ github.sha }}") {
		t.Error("main-head guard must compare github.sha to origin/main")
	}
}

// TestReleaseKeepsPublishSteps guards against a guard edit dropping the
// attestation, SBOM, scan, or Release steps.
func TestReleaseKeepsPublishSteps(t *testing.T) {
	text := stripComments(readFile(t, releaseWorkflow))
	for _, want := range []string{
		"actions/attest-build-provenance@",
		"anchore/sbom-action@",
		"actions/attest@",
		"gh attestation verify",
		"aquasecurity/trivy-action@",
		"softprops/action-gh-release@",
		"files: dist/eshu-api-image-spdx.json",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("release.yml lost %q", want)
		}
	}
}

// TestSemverGuardRefusesNonSemverTags runs the real step script.
func TestSemverGuardRefusesNonSemverTags(t *testing.T) {
	script := stepRun(t, releaseStep(t, semverStep))
	cases := map[string]int{
		"v0.1.0":       0,
		"v10.20.30":    0,
		"v1.2.3-rc.1":  0,
		"vfoo":         1,
		"v1.2":         1,
		"1.2.3":        1,
		"v1.2.3+build": 1,
		"v1.2.3-":      1,
		"v1.2.3 x":     1,
	}
	for tag, want := range cases {
		if got := runScript(t, t.TempDir(), script, "TAG_NAME="+tag); got != want {
			t.Errorf("tag %q: exit %d, want %d", tag, got, want)
		}
	}
}

// TestMainHeadGuardAcceptsOnlyMainHead runs the real step script against a
// scratch origin: only the commit at origin/main's head passes.
func TestMainHeadGuardAcceptsOnlyMainHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	git(t, root, "init", "--bare", "-b", "main", origin)
	git(t, root, "clone", "-q", origin, work)
	git(t, work, "checkout", "-q", "-b", "main")
	git(t, work, "commit", "-q", "--allow-empty", "-m", "first")
	older := git(t, work, "rev-parse", "HEAD")
	git(t, work, "commit", "-q", "--allow-empty", "-m", "second")
	head := git(t, work, "rev-parse", "HEAD")
	git(t, work, "push", "-q", "origin", "main")
	git(t, work, "checkout", "-q", "-b", "feature", older)
	git(t, work, "commit", "-q", "--allow-empty", "-m", "off main")
	offMain := git(t, work, "rev-parse", "HEAD")

	script := stepRun(t, releaseStep(t, mainHeadStep))
	cases := []struct {
		name   string
		commit string
		want   int
	}{
		{"main head", head, 0},
		{"older main commit", older, 1},
		{"off-main commit", offMain, 1},
	}
	for _, tc := range cases {
		got := runScript(t, work, script, "TAG_NAME=v0.1.0", "TAG_COMMIT="+tc.commit)
		if got != tc.want {
			t.Errorf("%s: exit %d, want %d", tc.name, got, tc.want)
		}
	}
}
