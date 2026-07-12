package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Configuration
const (
	templateDir = "templates"
	sopsFile    = "vars.yml"
	outputBase  = "output"
	isoName     = "ignition.iso"
)

// files maps a template name to its rendered output path (relative to outputBase).
var files = map[string]string{
	"config.ign.template": "ignition/config.ign",
	"script.template":     "combustion/script",
}

var funcMap = template.FuncMap{
	"toJson": func(v interface{}) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	},
	// trim mirrors Python's str.strip() used as `git_config_raw.strip()`.
	"trim": func(v interface{}) string {
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	},
}

func main() {
	if err := build(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func build() error {
	// Verify external tools are installed before doing any work.
	if err := checkDependencies("sops", "mkisofs"); err != nil {
		return err
	}

	if _, err := os.Stat(sopsFile); os.IsNotExist(err) {
		return fmt.Errorf("%s not found", sopsFile)
	}

	// Decrypt vars.yml in-memory via SOPS (no plaintext written to disk).
	cmd := exec.Command("sops", "decrypt", sopsFile)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	decrypted, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("SOPS decryption failed:\n%s", strings.TrimSpace(stderr.String()))
	}

	// Parse the decrypted YAML into a generic map for template rendering.
	var configVars map[string]interface{}
	if err := yaml.Unmarshal(decrypted, &configVars); err != nil {
		return fmt.Errorf("parsing decrypted YAML: %w", err)
	}

	fmt.Println("\nGenerating configuration files...")
	for tmplName, relPath := range files {
		fullPath := filepath.Join(outputBase, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", fullPath, err)
		}

		tmpl, err := template.New(tmplName).
		Funcs(funcMap).
		Option("missingkey=error").
		ParseFiles(filepath.Join(templateDir, tmplName))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", tmplName, err)
		}

		out, err := os.Create(fullPath)
		if err != nil {
			return fmt.Errorf("creating %s: %w", fullPath, err)
		}

		if err := tmpl.Execute(out, configVars); err != nil {
			out.Close()
			return fmt.Errorf("rendering %s: %w", tmplName, err)
		}
		out.Close()
		fmt.Printf("  - Created: %s\n", fullPath)
	}

	return createISO()
}

func createISO() error {
	fmt.Printf("\nBuilding %s...\n", isoName)

	cmd := exec.Command(
		"mkisofs",
		"-full-iso9660-filenames",
		"-o", isoName,
		"-V", "ignition",
		"./"+outputBase,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating ISO: %w", err)
	}

	fmt.Printf("Successfully created %s\n", isoName)
	return nil
}

// checkDependencies verifies that each required external command is on PATH.
func checkDependencies(tools ...string) error {
	var missing []string
	for _, t := range tools {
		if _, err := exec.LookPath(t); err != nil {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"required tool(s) not found on PATH: %s\nInstall them (e.g. `sudo zypper install %s`) and try again",
				  strings.Join(missing, ", "),
				  strings.Join(missing, " "),
		)
	}
	return nil
}
